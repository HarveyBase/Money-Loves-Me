package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/internal/logger"
	"money-loves-me/pkg/binance"
)

// 支持的K线时间间隔。
var SupportedIntervals = []string{"1m", "5m", "15m", "1h", "4h", "1d"}

// restPollInterval 是回退到 REST API 时的轮询间隔。
const restPollInterval = 5 * time.Second

// DataConsumer 是订阅者必须实现的接口，用于通过发布-订阅模式
// 接收实时市场数据更新。
type DataConsumer interface {
	OnKlineUpdate(symbol string, kline binance.Kline)
	OnOrderBookUpdate(symbol string, book *binance.OrderBook)
}

// MarketDataService 管理实时市场数据的获取和分发。
// 它使用 WebSocket 作为主要数据源，当 WebSocket 连接不可用时
// 回退到 REST API 轮询。
type MarketDataService struct {
	client *binance.BinanceClient
	ws     *binance.WSManager
	log    *logger.Logger

	mu          sync.RWMutex
	subscribers map[string][]DataConsumer     // 交易对 -> 消费者列表
	klineCache  map[string][]binance.Kline    // "交易对:时间间隔" -> K线数据
	orderBooks  map[string]*binance.OrderBook // 交易对 -> 订单簿

	// 回退轮询
	pollCtx    context.Context
	pollCancel context.CancelFunc
	polling    map[string]bool // 交易对 -> REST 轮询是否活跃
	pollMu     sync.Mutex
}

// NewMarketDataService 创建一个新的 MarketDataService。
func NewMarketDataService(client *binance.BinanceClient, ws *binance.WSManager, log *logger.Logger) *MarketDataService {
	ctx, cancel := context.WithCancel(context.Background())
	return &MarketDataService{
		client:      client,
		ws:          ws,
		log:         log,
		subscribers: make(map[string][]DataConsumer),
		klineCache:  make(map[string][]binance.Kline),
		orderBooks:  make(map[string]*binance.OrderBook),
		pollCtx:     ctx,
		pollCancel:  cancel,
		polling:     make(map[string]bool),
	}
}

// Subscribe 为指定交易对注册一个 DataConsumer。它会设置 WebSocket
// 订阅以获取K线（默认1分钟）和订单簿数据。如果 WebSocket 不可用，
// 则启动 REST API 轮询作为回退方案。
func (s *MarketDataService) Subscribe(symbol string, consumer DataConsumer) error {
	if consumer == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "consumer must not be nil", "market", nil)
	}
	if symbol == "" {
		return apperrors.NewAppError(apperrors.ErrValidation, "symbol must not be empty", "market", nil)
	}

	s.mu.Lock()
	existing := s.subscribers[symbol]
	// 防止重复订阅。
	for _, c := range existing {
		if c == consumer {
			s.mu.Unlock()
			return nil
		}
	}
	firstSubscriber := len(existing) == 0
	s.subscribers[symbol] = append(existing, consumer)
	s.mu.Unlock()

	// 仅为某个交易对的第一个订阅者设置 WebSocket 订阅。
	if firstSubscriber {
		if err := s.setupWebSocketSubscriptions(symbol); err != nil {
			s.log.Warn("websocket subscription failed, falling back to REST polling",
				zap.String("symbol", symbol), zap.Error(err))
			s.startPolling(symbol)
		}
	}

	return nil
}

// Unsubscribe 从指定交易对的订阅者列表中移除一个 DataConsumer。
// 如果该交易对没有剩余订阅者，则清理 WebSocket 订阅和轮询。
func (s *MarketDataService) Unsubscribe(symbol string, consumer DataConsumer) error {
	if consumer == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "consumer must not be nil", "market", nil)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[symbol]
	for i, c := range subs {
		if c == consumer {
			s.subscribers[symbol] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// 如果没有剩余订阅者则清理资源。
	if len(s.subscribers[symbol]) == 0 {
		delete(s.subscribers, symbol)
		s.stopPolling(symbol)
	}

	return nil
}

// GetHistoricalKlines 通过 REST API 获取指定交易对和时间间隔的历史K线数据。
func (s *MarketDataService) GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error) {
	if !isValidInterval(interval) {
		return nil, apperrors.NewAppError(apperrors.ErrValidation,
			fmt.Sprintf("unsupported interval %q, supported: %v", interval, SupportedIntervals),
			"market", nil)
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	klines, err := s.client.GetKlines(symbol, interval, startMs, endMs)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to get historical klines", "market", err)
	}

	// 使用最新数据更新缓存。
	cacheKey := klineCacheKey(symbol, interval)
	s.mu.Lock()
	s.klineCache[cacheKey] = klines
	s.mu.Unlock()

	return klines, nil
}

// GetCurrentPrice 从K线缓存中返回指定交易对的最新收盘价。
// 它检查所有缓存的时间间隔并返回最近的价格。
func (s *MarketDataService) GetCurrentPrice(symbol string) (decimal.Decimal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 尝试从任何缓存的时间间隔中查找最新价格。
	var latestPrice decimal.Decimal
	var latestTime time.Time

	for _, interval := range SupportedIntervals {
		key := klineCacheKey(symbol, interval)
		klines, ok := s.klineCache[key]
		if !ok || len(klines) == 0 {
			continue
		}
		last := klines[len(klines)-1]
		if last.CloseTime.After(latestTime) {
			latestTime = last.CloseTime
			latestPrice = last.Close
		}
	}

	if latestTime.IsZero() {
		// 没有缓存数据；尝试使用订单簿中间价。
		book, ok := s.orderBooks[symbol]
		if ok && len(book.Bids) > 0 && len(book.Asks) > 0 {
			mid := book.Bids[0].Price.Add(book.Asks[0].Price).Div(decimal.NewFromInt(2))
			return mid, nil
		}
		return decimal.Zero, apperrors.NewAppError(apperrors.ErrValidation,
			fmt.Sprintf("no price data available for %s", symbol), "market", nil)
	}

	return latestPrice, nil
}

// GetOrderBook 返回指定交易对的缓存订单簿。
func (s *MarketDataService) GetOrderBook(symbol string) (*binance.OrderBook, error) {
	s.mu.RLock()
	book, ok := s.orderBooks[symbol]
	s.mu.RUnlock()

	if !ok {
		// 如果未缓存，则从 REST API 获取。
		fetched, err := s.client.GetOrderBook(symbol, 20)
		if err != nil {
			return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to get order book", "market", err)
		}
		s.mu.Lock()
		s.orderBooks[symbol] = fetched
		s.mu.Unlock()
		return fetched, nil
	}

	return book, nil
}

// Close 关闭所有轮询协程并清理资源。
func (s *MarketDataService) Close() {
	s.pollCancel()
}

// --- WebSocket 订阅设置 ---

// setupWebSocketSubscriptions 通过 WebSocket 订阅K线和订单簿数据流。
func (s *MarketDataService) setupWebSocketSubscriptions(symbol string) error {
	// 订阅1分钟K线以获取实时价格更新。
	if err := s.ws.SubscribeKline(symbol, "1m", s.handleKlineEvent); err != nil {
		return err
	}

	// 订阅订单簿深度数据。
	if err := s.ws.SubscribeOrderBook(symbol, s.handleOrderBookEvent); err != nil {
		return err
	}

	return nil
}

// handleKlineEvent 处理传入的 WebSocket K线事件，更新缓存，
// 并通知所有订阅者。
func (s *MarketDataService) handleKlineEvent(event *binance.WsKlineEvent) {
	if event == nil {
		return
	}

	cacheKey := klineCacheKey(event.Symbol, event.Interval)

	s.mu.Lock()
	klines := s.klineCache[cacheKey]
	// 追加或更新最新的K线。
	if len(klines) > 0 && klines[len(klines)-1].OpenTime.Equal(event.Kline.OpenTime) {
		klines[len(klines)-1] = event.Kline
	} else {
		klines = append(klines, event.Kline)
		// 保持缓存有界（最近1000条K线）。
		if len(klines) > 1000 {
			klines = klines[len(klines)-1000:]
		}
	}
	s.klineCache[cacheKey] = klines

	// 获取订阅者快照。
	consumers := make([]DataConsumer, len(s.subscribers[event.Symbol]))
	copy(consumers, s.subscribers[event.Symbol])
	s.mu.Unlock()

	// 在锁外通知订阅者。
	for _, c := range consumers {
		c.OnKlineUpdate(event.Symbol, event.Kline)
	}
}

// handleOrderBookEvent 处理传入的 WebSocket 订单簿事件，更新缓存，
// 并通知所有订阅者。
func (s *MarketDataService) handleOrderBookEvent(event *binance.WsOrderBookEvent) {
	if event == nil {
		return
	}

	s.mu.Lock()
	s.orderBooks[event.Symbol] = &event.Book

	consumers := make([]DataConsumer, len(s.subscribers[event.Symbol]))
	copy(consumers, s.subscribers[event.Symbol])
	s.mu.Unlock()

	for _, c := range consumers {
		c.OnOrderBookUpdate(event.Symbol, &event.Book)
	}
}

// --- REST API 回退轮询 ---

// startPolling 当 WebSocket 不可用时，为指定交易对启动 REST API 轮询作为回退方案。
func (s *MarketDataService) startPolling(symbol string) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()

	if s.polling[symbol] {
		return
	}
	s.polling[symbol] = true

	go s.pollLoop(symbol)
}

// stopPolling 停止指定交易对的 REST API 轮询。
func (s *MarketDataService) stopPolling(symbol string) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	delete(s.polling, symbol)
}

// pollLoop 定期通过 REST API 获取K线和订单簿数据。
func (s *MarketDataService) pollLoop(symbol string) {
	ticker := time.NewTicker(restPollInterval)
	defer ticker.Stop()

	s.log.Info("started REST polling fallback", zap.String("symbol", symbol))

	for {
		select {
		case <-s.pollCtx.Done():
			return
		case <-ticker.C:
			// 检查该交易对的轮询是否仍然活跃。
			s.pollMu.Lock()
			active := s.polling[symbol]
			s.pollMu.Unlock()
			if !active {
				return
			}

			// 检查 WebSocket 是否已重新连接；如果是，则停止轮询。
			if s.ws.IsConnected() {
				s.log.Info("websocket reconnected, stopping REST polling",
					zap.String("symbol", symbol))
				if err := s.setupWebSocketSubscriptions(symbol); err != nil {
					s.log.Warn("failed to re-setup websocket after reconnect",
						zap.String("symbol", symbol), zap.Error(err))
					continue // 继续轮询
				}
				s.stopPolling(symbol)
				return
			}

			s.pollOnce(symbol)
		}
	}
}

// pollOnce 通过 REST API 获取最新的K线和订单簿数据，
// 并分发给订阅者。
func (s *MarketDataService) pollOnce(symbol string) {
	// 获取最新的1分钟K线。
	now := time.Now()
	start := now.Add(-2 * time.Minute)
	klines, err := s.client.GetKlines(symbol, "1m", start.UnixMilli(), now.UnixMilli())
	if err != nil {
		s.log.Warn("REST poll klines failed", zap.String("symbol", symbol), zap.Error(err))
	} else if len(klines) > 0 {
		latest := klines[len(klines)-1]
		cacheKey := klineCacheKey(symbol, "1m")

		s.mu.Lock()
		cached := s.klineCache[cacheKey]
		if len(cached) > 0 && cached[len(cached)-1].OpenTime.Equal(latest.OpenTime) {
			cached[len(cached)-1] = latest
		} else {
			cached = append(cached, latest)
			if len(cached) > 1000 {
				cached = cached[len(cached)-1000:]
			}
		}
		s.klineCache[cacheKey] = cached

		consumers := make([]DataConsumer, len(s.subscribers[symbol]))
		copy(consumers, s.subscribers[symbol])
		s.mu.Unlock()

		for _, c := range consumers {
			c.OnKlineUpdate(symbol, latest)
		}
	}

	// 获取订单簿。
	book, err := s.client.GetOrderBook(symbol, 20)
	if err != nil {
		s.log.Warn("REST poll order book failed", zap.String("symbol", symbol), zap.Error(err))
	} else {
		s.mu.Lock()
		s.orderBooks[symbol] = book

		consumers := make([]DataConsumer, len(s.subscribers[symbol]))
		copy(consumers, s.subscribers[symbol])
		s.mu.Unlock()

		for _, c := range consumers {
			c.OnOrderBookUpdate(symbol, book)
		}
	}
}

// --- 辅助函数 ---

// klineCacheKey 返回交易对和时间间隔组合的缓存键。
func klineCacheKey(symbol, interval string) string {
	return symbol + ":" + interval
}

// isValidInterval 检查给定的时间间隔是否受支持。
func isValidInterval(interval string) bool {
	for _, v := range SupportedIntervals {
		if v == interval {
			return true
		}
	}
	return false
}
