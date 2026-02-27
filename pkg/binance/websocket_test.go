package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"pgregory.net/rapid"

	"money-loves-me/internal/config"
	"money-loves-me/internal/logger"
)

// newTestLogger 为测试创建一个最小化的日志记录器。
func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.NewLogger("ws-test", config.LogConfig{Level: "DEBUG"})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return l
}

// upgrader 是测试服务器使用的默认升级器。
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// startEchoServer 启动一个 WebSocket 服务器，将消息回显并
// 对 SUBSCRIBE 请求返回确认。返回 ws:// URL 和清理函数。
func startEchoServer(t *testing.T) (string, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// 回显消息。
			_ = conn.WriteMessage(mt, msg)
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return wsURL, srv.Close
}

// startStreamServer 启动一个模拟 Binance 组合流格式的 WebSocket 服务器。
// 通过返回的 send 函数推送消息。
func startStreamServer(t *testing.T) (wsURL string, send func(stream string, data interface{}), cleanup func()) {
	t.Helper()

	var mu sync.Mutex
	var clients []*websocket.Conn

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		mu.Lock()
		clients = append(clients, conn)
		mu.Unlock()

		// 读取循环以保持连接活跃并处理订阅消息。
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	send = func(stream string, data interface{}) {
		payload, _ := json.Marshal(data)
		envelope := map[string]interface{}{
			"stream": stream,
			"data":   json.RawMessage(payload),
		}
		msg, _ := json.Marshal(envelope)

		mu.Lock()
		defer mu.Unlock()
		for _, c := range clients {
			_ = c.WriteMessage(websocket.TextMessage, msg)
		}
	}

	cleanup = func() {
		mu.Lock()
		for _, c := range clients {
			_ = c.Close()
		}
		mu.Unlock()
		srv.Close()
	}

	return wsURL, send, cleanup
}

func TestWSManager_ConnectAndClose(t *testing.T) {
	wsURL, cleanup := startEchoServer(t)
	defer cleanup()

	mgr := NewWSManager(wsURL, newTestLogger(t))

	if err := mgr.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !mgr.IsConnected() {
		t.Fatal("expected IsConnected to be true after Connect")
	}

	if err := mgr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if mgr.IsConnected() {
		t.Fatal("expected IsConnected to be false after Close")
	}
}

func TestWSManager_SubscribeKline(t *testing.T) {
	wsURL, send, cleanup := startStreamServer(t)
	defer cleanup()

	mgr := NewWSManager(wsURL, newTestLogger(t))
	defer mgr.Close()

	var received atomic.Int32

	err := mgr.SubscribeKline("BTCUSDT", "1m", func(event *WsKlineEvent) {
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("SubscribeKline failed: %v", err)
	}

	// 等待连接建立。
	time.Sleep(100 * time.Millisecond)

	// 发送一个 K 线事件。
	send("btcusdt@kline_1m", map[string]interface{}{
		"s": "BTCUSDT",
		"k": map[string]interface{}{
			"i": "1m",
			"o": "42000.00",
			"h": "42100.00",
			"l": "41900.00",
			"c": "42050.00",
			"v": "100.5",
			"t": time.Now().UnixMilli(),
			"T": time.Now().Add(time.Minute).UnixMilli(),
		},
	})

	// 等待处理器被调用。
	time.Sleep(200 * time.Millisecond)

	if received.Load() == 0 {
		t.Fatal("expected kline handler to be called at least once")
	}

	subs := mgr.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
}

func TestWSManager_SubscribeOrderBook(t *testing.T) {
	wsURL, send, cleanup := startStreamServer(t)
	defer cleanup()

	mgr := NewWSManager(wsURL, newTestLogger(t))
	defer mgr.Close()

	var received atomic.Int32

	err := mgr.SubscribeOrderBook("ETHUSDT", func(event *WsOrderBookEvent) {
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("SubscribeOrderBook failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	send("ethusdt@depth20@100ms", map[string]interface{}{
		"bids": [][]string{{"3000.00", "1.5"}},
		"asks": [][]string{{"3001.00", "2.0"}},
	})

	time.Sleep(200 * time.Millisecond)

	if received.Load() == 0 {
		t.Fatal("expected order book handler to be called at least once")
	}
}

func TestWSManager_SubscribeUserData(t *testing.T) {
	wsURL, send, cleanup := startStreamServer(t)
	defer cleanup()

	mgr := NewWSManager(wsURL, newTestLogger(t))
	defer mgr.Close()

	var received atomic.Int32

	err := mgr.SubscribeUserData(func(event *WsUserDataEvent) {
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("SubscribeUserData failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	send("userData", map[string]interface{}{
		"e": "executionReport",
		"s": "BTCUSDT",
	})

	time.Sleep(200 * time.Millisecond)

	if received.Load() == 0 {
		t.Fatal("expected user data handler to be called at least once")
	}
}

func TestWSManager_NilHandlerRejected(t *testing.T) {
	mgr := NewWSManager("ws://localhost:0", newTestLogger(t))

	if err := mgr.SubscribeKline("BTC", "1m", nil); err == nil {
		t.Fatal("expected error for nil kline handler")
	}
	if err := mgr.SubscribeOrderBook("BTC", nil); err == nil {
		t.Fatal("expected error for nil order book handler")
	}
	if err := mgr.SubscribeUserData(nil); err == nil {
		t.Fatal("expected error for nil user data handler")
	}
}

func TestWSManager_Reconnect(t *testing.T) {
	// 启动一个服务器，订阅，然后关闭服务器并重启。
	// 管理器应自动重连并重新订阅。

	var mu sync.Mutex
	var serverConn *websocket.Conn
	var subscribeCount atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		serverConn = conn
		mu.Unlock()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// 统计 SUBSCRIBE 消息。
			var req map[string]interface{}
			if json.Unmarshal(msg, &req) == nil {
				if method, ok := req["method"].(string); ok && method == "SUBSCRIBE" {
					subscribeCount.Add(1)
				}
			}
		}
	})

	srv := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	mgr := NewWSManager(wsURL, newTestLogger(t))
	defer mgr.Close()

	err := mgr.SubscribeKline("BTCUSDT", "1m", func(event *WsKlineEvent) {})
	if err != nil {
		t.Fatalf("SubscribeKline failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	initialSubs := subscribeCount.Load()
	if initialSubs == 0 {
		t.Fatal("expected at least one SUBSCRIBE message")
	}

	// 关闭服务端连接以触发重连。
	mu.Lock()
	if serverConn != nil {
		_ = serverConn.Close()
	}
	mu.Unlock()

	// 等待重连 + 重新订阅（基础延迟为 3 秒，所以多等一会儿）。
	time.Sleep(5 * time.Second)

	finalSubs := subscribeCount.Load()
	if finalSubs <= initialSubs {
		t.Fatalf("expected resubscribe after reconnect: initial=%d, final=%d", initialSubs, finalSubs)
	}
}

// Feature: binance-trading-system, Property 16: WebSocket 断线重订阅
// **Validates: Requirements 2.6**
//
// 对于任意已订阅的流集合，在 WebSocket 断开并重连后，
// 所有之前订阅的流应被自动重新订阅。
func TestProperty16_WebSocketReconnectResubscribe(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// --- 生成一组随机订阅 ---

		// 生成 1-5 个随机交易对和时间间隔的 K 线订阅。
		validIntervals := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
		numKlines := rapid.IntRange(0, 5).Draw(rt, "numKlines")
		numOrderBooks := rapid.IntRange(0, 3).Draw(rt, "numOrderBooks")
		includeUserData := rapid.Bool().Draw(rt, "includeUserData")

		type klineSub struct {
			symbol   string
			interval string
		}

		// 构建唯一的 K 线订阅。
		klineSubs := make([]klineSub, 0, numKlines)
		klineStreams := make(map[string]bool)
		for i := 0; i < numKlines; i++ {
			sym := strings.ToUpper(rapid.StringMatching(`[A-Z]{3,6}`).Draw(rt, fmt.Sprintf("klineSymbol_%d", i)))
			intv := validIntervals[rapid.IntRange(0, len(validIntervals)-1).Draw(rt, fmt.Sprintf("klineInterval_%d", i))]
			stream := strings.ToLower(sym) + "@kline_" + intv
			if klineStreams[stream] {
				continue // 跳过重复项
			}
			klineStreams[stream] = true
			klineSubs = append(klineSubs, klineSub{symbol: sym, interval: intv})
		}

		// 构建唯一的订单簿订阅。
		obSymbols := make([]string, 0, numOrderBooks)
		obStreams := make(map[string]bool)
		for i := 0; i < numOrderBooks; i++ {
			sym := strings.ToUpper(rapid.StringMatching(`[A-Z]{3,6}`).Draw(rt, fmt.Sprintf("obSymbol_%d", i)))
			stream := strings.ToLower(sym) + "@depth20@100ms"
			if obStreams[stream] {
				continue
			}
			obStreams[stream] = true
			obSymbols = append(obSymbols, sym)
		}

		totalExpected := len(klineSubs) + len(obSymbols)
		if includeUserData {
			totalExpected++
		}

		// 至少需要 1 个订阅才能测试重连行为。
		if totalExpected == 0 {
			return
		}

		// --- 设置一个跟踪 SUBSCRIBE 消息的测试服务器 ---

		var mu sync.Mutex
		var serverConns []*websocket.Conn
		var subscribeMessages []string // 收集所有 SUBSCRIBE 请求中的流名称

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			mu.Lock()
			serverConns = append(serverConns, conn)
			mu.Unlock()

			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var req struct {
					Method string   `json:"method"`
					Params []string `json:"params"`
				}
				if json.Unmarshal(msg, &req) == nil && req.Method == "SUBSCRIBE" {
					mu.Lock()
					subscribeMessages = append(subscribeMessages, req.Params...)
					mu.Unlock()
				}
			}
		})

		srv := httptest.NewServer(handler)
		defer srv.Close()
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

		mgr := NewWSManager(wsURL, newTestLogger(t))
		defer mgr.Close()

		// --- 订阅所有生成的流 ---

		for _, ks := range klineSubs {
			err := mgr.SubscribeKline(ks.symbol, ks.interval, func(event *WsKlineEvent) {})
			if err != nil {
				t.Fatalf("SubscribeKline(%s, %s) failed: %v", ks.symbol, ks.interval, err)
			}
		}
		for _, sym := range obSymbols {
			err := mgr.SubscribeOrderBook(sym, func(event *WsOrderBookEvent) {})
			if err != nil {
				t.Fatalf("SubscribeOrderBook(%s) failed: %v", sym, err)
			}
		}
		if includeUserData {
			err := mgr.SubscribeUserData(func(event *WsUserDataEvent) {})
			if err != nil {
				t.Fatalf("SubscribeUserData failed: %v", err)
			}
		}

		// 等待初始订阅发送完成。
		time.Sleep(300 * time.Millisecond)

		// 记录断开连接前的订阅集合。
		subsBeforeDisconnect := mgr.Subscriptions()
		if len(subsBeforeDisconnect) != totalExpected {
			t.Fatalf("expected %d subscriptions before disconnect, got %d", totalExpected, len(subsBeforeDisconnect))
		}

		// 记录初始订阅计数。
		mu.Lock()
		initialSubCount := len(subscribeMessages)
		mu.Unlock()

		if initialSubCount < totalExpected {
			t.Fatalf("expected at least %d initial SUBSCRIBE params, got %d", totalExpected, initialSubCount)
		}

		// --- 通过关闭所有服务端连接来模拟断开连接 ---
		mu.Lock()
		for _, c := range serverConns {
			_ = c.Close()
		}
		serverConns = nil
		mu.Unlock()

		// 等待重连 + 重新订阅。
		// 基础延迟为 3 秒，所以等待足够长的时间以完成至少一次重连尝试。
		time.Sleep(5 * time.Second)

		// --- 验证：所有流都已重新订阅 ---

		mu.Lock()
		totalSubParams := len(subscribeMessages)
		// 收集重新订阅的流（初始批次之后的那些）。
		resubscribedStreams := make(map[string]bool)
		for i := initialSubCount; i < totalSubParams; i++ {
			resubscribedStreams[subscribeMessages[i]] = true
		}
		mu.Unlock()

		// 重新订阅的集合应包含所有原始流。
		expectedStreams := make(map[string]bool)
		for stream := range klineStreams {
			expectedStreams[stream] = true
		}
		for stream := range obStreams {
			expectedStreams[stream] = true
		}
		if includeUserData {
			expectedStreams["userData"] = true
		}

		for stream := range expectedStreams {
			if !resubscribedStreams[stream] {
				t.Fatalf("stream %q was not resubscribed after reconnect", stream)
			}
		}

		// 同时验证管理器上的订阅集合未发生变化。
		subsAfterReconnect := mgr.Subscriptions()
		if len(subsAfterReconnect) != totalExpected {
			t.Fatalf("expected %d subscriptions after reconnect, got %d", totalExpected, len(subsAfterReconnect))
		}

		afterSet := make(map[string]bool)
		for _, s := range subsAfterReconnect {
			afterSet[s] = true
		}
		for _, s := range subsBeforeDisconnect {
			if !afterSet[s] {
				t.Fatalf("subscription %q lost after reconnect", s)
			}
		}
	})
}
