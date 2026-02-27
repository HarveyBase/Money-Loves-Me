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
	// wsPingInterval is how often we send a ping frame to keep the connection alive.
	wsPingInterval = 30 * time.Second
	// wsReadTimeout is the maximum time to wait for a pong or any message before
	// considering the connection dead.
	wsReadTimeout = 60 * time.Second
	// wsReconnectBaseDelay is the initial delay before the first reconnect attempt.
	wsReconnectBaseDelay = 3 * time.Second
	// wsMaxReconnectRetries is the maximum number of consecutive reconnect attempts.
	wsMaxReconnectRetries = 5
)

// KlineHandler is called when a kline update is received.
type KlineHandler func(event *WsKlineEvent)

// OrderBookHandler is called when an order book update is received.
type OrderBookHandler func(event *WsOrderBookEvent)

// UserDataHandler is called when a user data event is received.
type UserDataHandler func(event *WsUserDataEvent)

// WsKlineEvent represents a kline/candlestick WebSocket event.
type WsKlineEvent struct {
	Symbol   string `json:"s"`
	Interval string `json:"i"`
	Kline    Kline
}

// WsOrderBookEvent represents a depth/order book WebSocket event.
type WsOrderBookEvent struct {
	Symbol string `json:"s"`
	Book   OrderBook
}

// WsUserDataEvent represents a user data stream event (order updates, balance changes, etc.).
type WsUserDataEvent struct {
	EventType string          `json:"e"`
	RawData   json.RawMessage `json:"-"`
}

// subscriptionKind identifies the type of a subscription.
type subscriptionKind int

const (
	subKline subscriptionKind = iota
	subOrderBook
	subUserData
)

// subscription stores everything needed to re-subscribe after a reconnect.
type subscription struct {
	kind     subscriptionKind
	stream   string // e.g. "btcusdt@kline_1m"
	symbol   string
	interval string // only for kline
	klineH   KlineHandler
	bookH    OrderBookHandler
	userH    UserDataHandler
}

// WSManager manages a single multiplexed WebSocket connection to Binance,
// handling heartbeat, timeout detection, subscriptions, and auto-reconnect.
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

// NewWSManager creates a new WebSocket manager. It does NOT connect immediately;
// the connection is established lazily on the first subscription or by calling Connect.
func NewWSManager(wsURL string, log *logger.Logger) *WSManager {
	return &WSManager{
		baseURL: wsURL,
		log:     log,
		subs:    make(map[string]*subscription),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection and starts the read/heartbeat loops.
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

// Close gracefully shuts down the WebSocket connection and stops all loops.
func (m *WSManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.stopCh:
		// already closed
		return nil
	default:
		close(m.stopCh)
	}

	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	m.isConnected = false

	// Wait for loops to finish.
	<-m.done
	return nil
}

// IsConnected reports whether the WebSocket connection is currently active.
func (m *WSManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// Subscriptions returns a snapshot of the current stream names.
func (m *WSManager) Subscriptions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.subs))
	for s := range m.subs {
		names = append(names, s)
	}
	return names
}

// SubscribeKline subscribes to kline updates for the given symbol and interval.
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

// SubscribeOrderBook subscribes to order book depth updates for the given symbol.
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

// SubscribeUserData subscribes to the user data stream using the given listen key.
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

// --- internal methods ---

// dialLocked establishes the raw WebSocket connection. Caller must hold m.mu.
func (m *WSManager) dialLocked() error {
	// Build the combined stream URL.
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

// streamNames returns the list of stream names from current subscriptions.
// Caller must hold m.mu (at least RLock).
func (m *WSManager) streamNames() []string {
	names := make([]string, 0, len(m.subs))
	for s := range m.subs {
		names = append(names, s)
	}
	return names
}

// sendSubscribe sends a SUBSCRIBE request over the WebSocket for the given stream.
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

// resubscribeAll sends SUBSCRIBE requests for every registered subscription.
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

// heartbeatLoop sends periodic ping frames and is responsible for detecting timeouts.
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

// readLoop reads messages from the WebSocket and dispatches them to handlers.
// When the connection drops it triggers reconnection.
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

		// Set read deadline for timeout detection.
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

// dispatch routes a raw WebSocket message to the appropriate handler.
func (m *WSManager) dispatch(raw []byte) {
	// Binance combined stream format: {"stream":"<stream>","data":{...}}
	var envelope struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		// Might be a direct (non-combined) message or a subscription ack.
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

// handleKlineMessage parses and dispatches a kline event.
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

// handleOrderBookMessage parses and dispatches an order book event.
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

// handleUserDataMessage parses and dispatches a user data event.
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

// handleDisconnect marks the connection as dead and attempts to reconnect
// with exponential backoff.
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

		// Exponential backoff: baseDelay * 2^attempt, capped at baseDelay.
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

		// Connection re-established — resubscribe all streams.
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
