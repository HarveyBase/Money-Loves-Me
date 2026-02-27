package binance

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/internal/logger"
)

const (
	// wsPingInterval 是发送 ping 帧以保持连接活跃的频率。
	wsPingInterval = 30 * time.Second
	// wsReadTimeout 是等待 pong 或任何消息的最大时间，
	// 超过此时间则认为连接已断开。
	wsReadTimeout = 60 * time.Second
	// wsReconnectBaseDelay 是首次重连尝试前的初始延迟。
	wsReconnectBaseDelay = 3 * time.Second
	// wsMaxReconnectRetries 是连续重连尝试的最大次数。
	wsMaxReconnectRetries = 5
)

// KlineHandler 在收到 K 线更新时被调用。
type KlineHandler func(event *WsKlineEvent)

// OrderBookHandler 在收到订单簿更新时被调用。
type OrderBookHandler func(event *WsOrderBookEvent)

// UserDataHandler 在收到用户数据事件时被调用。
type UserDataHandler func(event *WsUserDataEvent)

// WsKlineEvent 表示一个 K 线/蜡烛图 WebSocket 事件。
type WsKlineEvent struct {
	Symbol   string `json:"s"`
	Interval string `json:"i"`
	Kline    Kline
}

// WsOrderBookEvent 表示一个深度/订单簿 WebSocket 事件。
type WsOrderBookEvent struct {
	Symbol string `json:"s"`
	Book   OrderBook
}

// WsUserDataEvent 表示一个用户数据流事件（订单更新、余额变动等）。
type WsUserDataEvent struct {
	EventType string          `json:"e"`
	RawData   json.RawMessage `json:"-"`
}

// subscriptionKind 标识订阅的类型。
type subscriptionKind int

const (
	subKline subscriptionKind = iota
	subOrderBook
	subUserData
)

// subscription 存储重连后重新订阅所需的全部信息。
type subscription struct {
	kind     subscriptionKind
	stream   string // 例如 "btcusdt@kline_1m"
	symbol   string
	interval string // 仅用于 K 线
	klineH   KlineHandler
	bookH    OrderBookHandler
	userH    UserDataHandler
}

// WSManager 管理与 Binance 的单个多路复用 WebSocket 连接，
// 处理心跳、超时检测、订阅和自动重连。
type WSManager struct {
	baseURL string
	log     *logger.Logger

	mu          sync.RWMutex
	conn        *websocket.Conn
	subs        map[string]*subscription // stream -> subscription
	stopCh      chan struct{}
	done        chan struct{}
	isConnected bool
}

// NewWSManager 创建一个新的 WebSocket 管理器。它不会立即连接；
// 连接会在首次订阅时或调用 Connect 时延迟建立。
func NewWSManager(wsURL string, log *logger.Logger) *WSManager {
	return &WSManager{
		baseURL: wsURL,
		log:     log,
		subs:    make(map[string]*subscription),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Connect 建立 WebSocket 连接并启动读取/心跳循环。
func (m *WSManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isConnected {
		return nil
	}

	if err := m.dialLocked(); err != nil {
		return err
	}

	go m.readLoop()
	go m.heartbeatLoop()

	return nil
}

// Close 优雅地关闭 WebSocket 连接并停止所有循环。
func (m *WSManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.stopCh:
		// 已经关闭
		return nil
	default:
		close(m.stopCh)
	}

	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	m.isConnected = false

	// 等待循环结束。
	<-m.done
	return nil
}

// IsConnected 报告 WebSocket 连接当前是否处于活跃状态。
func (m *WSManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// Subscriptions 返回当前流名称的快照。
func (m *WSManager) Subscriptions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.subs))
	for s := range m.subs {
		names = append(names, s)
	}
	return names
}

// SubscribeKline 订阅指定交易对和时间间隔的 K 线更新。
func (m *WSManager) SubscribeKline(symbol, interval string, handler KlineHandler) error {
	if handler == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "kline handler must not be nil", "websocket", nil)
	}

	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
	sub := &subscription{
		kind:     subKline,
		stream:   stream,
		symbol:   symbol,
		interval: interval,
		klineH:   handler,
	}

	m.mu.Lock()
	m.subs[stream] = sub
	connected := m.isConnected
	m.mu.Unlock()

	if !connected {
		if err := m.Connect(); err != nil {
			return err
		}
	}

	return m.sendSubscribe(stream)
}

// SubscribeOrderBook 订阅指定交易对的订单簿深度更新。
func (m *WSManager) SubscribeOrderBook(symbol string, handler OrderBookHandler) error {
	if handler == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "order book handler must not be nil", "websocket", nil)
	}

	stream := fmt.Sprintf("%s@depth20@100ms", strings.ToLower(symbol))
	sub := &subscription{
		kind:   subOrderBook,
		stream: stream,
		symbol: symbol,
		bookH:  handler,
	}

	m.mu.Lock()
	m.subs[stream] = sub
	connected := m.isConnected
	m.mu.Unlock()

	if !connected {
		if err := m.Connect(); err != nil {
			return err
		}
	}

	return m.sendSubscribe(stream)
}

// SubscribeUserData 使用给定的监听密钥订阅用户数据流。
func (m *WSManager) SubscribeUserData(handler UserDataHandler) error {
	if handler == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "user data handler must not be nil", "websocket", nil)
	}

	stream := "userData"
	sub := &subscription{
		kind:   subUserData,
		stream: stream,
		userH:  handler,
	}

	m.mu.Lock()
	m.subs[stream] = sub
	connected := m.isConnected
	m.mu.Unlock()

	if !connected {
		if err := m.Connect(); err != nil {
			return err
		}
	}

	return m.sendSubscribe(stream)
}

// --- 内部方法 ---

// dialLocked 建立原始 WebSocket 连接。调用者必须持有 m.mu 锁。
func (m *WSManager) dialLocked() error {
	// 构建组合流 URL。
	streams := m.streamNames()
	wsEndpoint := m.baseURL + "/ws"
	if len(streams) > 0 {
		wsEndpoint = m.baseURL + "/stream?streams=" + strings.Join(streams, "/")
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsEndpoint, nil)
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrNetwork, "websocket dial failed", "websocket", err)
	}

	m.conn = conn
	m.isConnected = true
	m.log.Info("websocket connected", zap.String("url", wsEndpoint))
	return nil
}

// streamNames 返回当前订阅的流名称列表。
// 调用者必须持有 m.mu 锁（至少 RLock）。
func (m *WSManager) streamNames() []string {
	names := make([]string, 0, len(m.subs))
	for s := range m.subs {
		names = append(names, s)
	}
	return names
}

// sendSubscribe 通过 WebSocket 发送指定流的 SUBSCRIBE 请求。
func (m *WSManager) sendSubscribe(stream string) error {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	if conn == nil {
		return apperrors.NewAppError(apperrors.ErrNetwork, "not connected", "websocket", nil)
	}

	msg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().UnixNano(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.conn.WriteJSON(msg); err != nil {
		return apperrors.NewAppError(apperrors.ErrNetwork, "failed to send subscribe", "websocket", err)
	}
	m.log.Info("subscribed to stream", zap.String("stream", stream))
	return nil
}

// resubscribeAll 为每个已注册的订阅发送 SUBSCRIBE 请求。
func (m *WSManager) resubscribeAll() error {
	m.mu.RLock()
	streams := m.streamNames()
	m.mu.RUnlock()

	for _, s := range streams {
		if err := m.sendSubscribe(s); err != nil {
			return err
		}
	}
	return nil
}

// heartbeatLoop 定期发送 ping 帧，并负责检测超时。
func (m *WSManager) heartbeatLoop() {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			conn := m.conn
			m.mu.RUnlock()

			if conn == nil {
				continue
			}

			if err := conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(10*time.Second),
			); err != nil {
				m.log.Warn("ping failed", zap.Error(err))
			}
		}
	}
}

// readLoop 从 WebSocket 读取消息并将其分发给处理器。
// 当连接断开时触发重连。
func (m *WSManager) readLoop() {
	defer close(m.done)

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		m.mu.RLock()
		conn := m.conn
		m.mu.RUnlock()

		if conn == nil {
			return
		}

		// 设置读取截止时间用于超时检测。
		_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))

		_, message, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-m.stopCh:
				return
			default:
			}

			m.log.Warn("websocket read error, attempting reconnect", zap.Error(err))
			m.handleDisconnect()
			continue
		}

		m.dispatch(message)
	}
}

// dispatch 将原始 WebSocket 消息路由到相应的处理器。
func (m *WSManager) dispatch(raw []byte) {
	// Binance 组合流格式：{"stream":"<stream>","data":{...}}
	var envelope struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		// 可能是直接（非组合）消息或订阅确认。
		return
	}

	if envelope.Stream == "" {
		return
	}

	m.mu.RLock()
	sub, ok := m.subs[envelope.Stream]
	m.mu.RUnlock()

	if !ok {
		return
	}

	switch sub.kind {
	case subKline:
		m.handleKlineMessage(sub, envelope.Data)
	case subOrderBook:
		m.handleOrderBookMessage(sub, envelope.Data)
	case subUserData:
		m.handleUserDataMessage(sub, envelope.Data)
	}
}

// handleKlineMessage 解析并分发 K 线事件。
func (m *WSManager) handleKlineMessage(sub *subscription, data json.RawMessage) {
	var raw struct {
		Symbol string `json:"s"`
		Kline  struct {
			Interval  string `json:"i"`
			Open      string `json:"o"`
			High      string `json:"h"`
			Low       string `json:"l"`
			Close     string `json:"c"`
			Volume    string `json:"v"`
			OpenTime  int64  `json:"t"`
			CloseTime int64  `json:"T"`
		} `json:"k"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		m.log.Warn("failed to parse kline event", zap.Error(err))
		return
	}

	open, _ := toDecimal(raw.Kline.Open)
	high, _ := toDecimal(raw.Kline.High)
	low, _ := toDecimal(raw.Kline.Low)
	cl, _ := toDecimal(raw.Kline.Close)
	vol, _ := toDecimal(raw.Kline.Volume)

	event := &WsKlineEvent{
		Symbol:   raw.Symbol,
		Interval: raw.Kline.Interval,
		Kline: Kline{
			OpenTime:  time.UnixMilli(raw.Kline.OpenTime),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     cl,
			Volume:    vol,
			CloseTime: time.UnixMilli(raw.Kline.CloseTime),
		},
	}

	sub.klineH(event)
}

// handleOrderBookMessage 解析并分发订单簿事件。
func (m *WSManager) handleOrderBookMessage(sub *subscription, data json.RawMessage) {
	var raw struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		m.log.Warn("failed to parse order book event", zap.Error(err))
		return
	}

	event := &WsOrderBookEvent{
		Symbol: sub.symbol,
		Book: OrderBook{
			Symbol:     sub.symbol,
			Bids:       parsePriceLevels(raw.Bids),
			Asks:       parsePriceLevels(raw.Asks),
			UpdateTime: time.Now(),
		},
	}

	sub.bookH(event)
}

// handleUserDataMessage 解析并分发用户数据事件。
func (m *WSManager) handleUserDataMessage(sub *subscription, data json.RawMessage) {
	var raw struct {
		EventType string `json:"e"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		m.log.Warn("failed to parse user data event", zap.Error(err))
		return
	}

	event := &WsUserDataEvent{
		EventType: raw.EventType,
		RawData:   data,
	}

	sub.userH(event)
}

// handleDisconnect 将连接标记为已断开，并尝试使用指数退避策略进行重连。
func (m *WSManager) handleDisconnect() {
	m.mu.Lock()
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	m.isConnected = false
	m.mu.Unlock()

	m.log.Warn("websocket disconnected, starting reconnect")

	for attempt := 0; attempt < wsMaxReconnectRetries; attempt++ {
		select {
		case <-m.stopCh:
			return
		default:
		}

		// 指数退避：baseDelay * 2^attempt，上限为 baseDelay。
		delay := time.Duration(float64(wsReconnectBaseDelay) * math.Pow(2, float64(attempt)))
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}

		m.log.Info("reconnect attempt",
			zap.Int("attempt", attempt+1),
			zap.Int("maxRetries", wsMaxReconnectRetries),
			zap.Duration("delay", delay),
		)

		select {
		case <-m.stopCh:
			return
		case <-time.After(delay):
		}

		m.mu.Lock()
		err := m.dialLocked()
		m.mu.Unlock()

		if err != nil {
			m.log.Warn("reconnect failed", zap.Int("attempt", attempt+1), zap.Error(err))
			continue
		}

		// 连接已重新建立 - 重新订阅所有流。
		if err := m.resubscribeAll(); err != nil {
			m.log.Warn("resubscribe failed after reconnect", zap.Error(err))
			continue
		}

		m.log.Info("reconnected and resubscribed successfully")
		return
	}

	m.log.Error("all reconnect attempts exhausted",
		zap.Int("maxRetries", wsMaxReconnectRetries),
	)
}
