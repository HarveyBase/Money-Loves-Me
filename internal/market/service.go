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

// Supported kline intervals.
var SupportedIntervals = []string{"1m", "5m", "15m", "1h", "4h", "1d"}

// restPollInterval is the polling interval when falling back to REST API.
const restPollInterval = 5 * time.Second

// DataConsumer is the interface that subscribers must implement to receive
// real-time market data updates via the pub-sub pattern.
type DataConsumer interface {
	OnKlineUpdate(symbol string, kline binance.Kline)
	OnOrderBookUpdate(symbol string, book *binance.OrderBook)
}

// MarketDataService manages real-time market data acquisition and distribution.
// It uses WebSocket as the primary data source and falls back to REST API
// polling when the WebSocket connection is unavailable.
type MarketDataService struct {
	client *binance.BinanceClient
	ws     *binance.WSManager
	log    *logger.Logger

	mu          sync.RWMutex
	subscribers map[string][]DataConsumer     // symbol -> consumers
	klineCache  map[string][]binance.Kline    // "symbol:interval" -> klines
	orderBooks  map[string]*binance.OrderBook // symbol -> order book

	// fallback polling
	pollCtx    context.Context
	pollCancel context.CancelFunc
	polling    map[string]bool // symbol -> whether REST polling is active
	pollMu     sync.Mutex
}

// NewMarketDataService creates a new MarketDataService.
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

// Subscribe registers a DataConsumer for the given symbol. It sets up WebSocket
// subscriptions for kline (1m default) and order book data. If WebSocket is
// unavailable, it starts REST API polling as a fallback.
func (s *MarketDataService) Subscribe(symbol string, consumer DataConsumer) error {
	if consumer == nil {
		return apperrors.NewAppError(apperrors.ErrValidation, "consumer must not be nil", "market", nil)
	}
	if symbol == "" {
		return apperrors.NewAppError(apperrors.ErrValidation, "symbol must not be empty", "market", nil)
	}

	s.mu.Lock()
	existing := s.subscribers[symbol]
	// Prevent duplicate subscriptions.
	for _, c := range existing {
		if c == consumer {
			s.mu.Unlock()
			return nil
		}
	}
	firstSubscriber := len(existing) == 0
	s.subscribers[symbol] = append(existing, consumer)
	s.mu.Unlock()

	// Only set up WebSocket subscriptions for the first subscriber of a symbol.
	if firstSubscriber {
		if err := s.setupWebSocketSubscriptions(symbol); err != nil {
			s.log.Warn("websocket subscription failed, falling back to REST polling",
				zap.String("symbol", symbol), zap.Error(err))
			s.startPolling(symbol)
		}
	}

	return nil
}

// Unsubscribe removes a DataConsumer from the given symbol's subscriber list.
// If no subscribers remain for the symbol, WebSocket subscriptions and polling
// are cleaned up.
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

	// Clean up if no subscribers remain.
	if len(s.subscribers[symbol]) == 0 {
		delete(s.subscribers, symbol)
		s.stopPolling(symbol)
	}

	return nil
}

// GetHistoricalKlines fetches historical kline data for the given symbol and
// interval via the REST API.
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

	// Update cache with the latest data.
	cacheKey := klineCacheKey(symbol, interval)
	s.mu.Lock()
	s.klineCache[cacheKey] = klines
	s.mu.Unlock()

	return klines, nil
}

// GetCurrentPrice returns the latest close price for the given symbol from the
// kline cache. It checks all cached intervals and returns the most recent price.
func (s *MarketDataService) GetCurrentPrice(symbol string) (decimal.Decimal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try to find the latest price from any cached interval.
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
		// No cached data; try order book mid-price.
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

// GetOrderBook returns the cached order book for the given symbol.
func (s *MarketDataService) GetOrderBook(symbol string) (*binance.OrderBook, error) {
	s.mu.RLock()
	book, ok := s.orderBooks[symbol]
	s.mu.RUnlock()

	if !ok {
		// Fetch from REST API if not cached.
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

// Close shuts down all polling goroutines and cleans up resources.
func (s *MarketDataService) Close() {
	s.pollCancel()
}

// --- WebSocket subscription setup ---

// setupWebSocketSubscriptions subscribes to kline and order book streams via WebSocket.
func (s *MarketDataService) setupWebSocketSubscriptions(symbol string) error {
	// Subscribe to 1m kline for real-time price updates.
	if err := s.ws.SubscribeKline(symbol, "1m", s.handleKlineEvent); err != nil {
		return err
	}

	// Subscribe to order book depth.
	if err := s.ws.SubscribeOrderBook(symbol, s.handleOrderBookEvent); err != nil {
		return err
	}

	return nil
}

// handleKlineEvent processes incoming WebSocket kline events, updates the cache,
// and notifies all subscribers.
func (s *MarketDataService) handleKlineEvent(event *binance.WsKlineEvent) {
	if event == nil {
		return
	}

	cacheKey := klineCacheKey(event.Symbol, event.Interval)

	s.mu.Lock()
	klines := s.klineCache[cacheKey]
	// Append or update the latest kline.
	if len(klines) > 0 && klines[len(klines)-1].OpenTime.Equal(event.Kline.OpenTime) {
		klines[len(klines)-1] = event.Kline
	} else {
		klines = append(klines, event.Kline)
		// Keep cache bounded (last 1000 klines).
		if len(klines) > 1000 {
			klines = klines[len(klines)-1000:]
		}
	}
	s.klineCache[cacheKey] = klines

	// Get subscribers snapshot.
	consumers := make([]DataConsumer, len(s.subscribers[event.Symbol]))
	copy(consumers, s.subscribers[event.Symbol])
	s.mu.Unlock()

	// Notify subscribers outside the lock.
	for _, c := range consumers {
		c.OnKlineUpdate(event.Symbol, event.Kline)
	}
}

// handleOrderBookEvent processes incoming WebSocket order book events, updates
// the cache, and notifies all subscribers.
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

// --- REST API fallback polling ---

// startPolling begins REST API polling for a symbol as a fallback when
// WebSocket is unavailable.
func (s *MarketDataService) startPolling(symbol string) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()

	if s.polling[symbol] {
		return
	}
	s.polling[symbol] = true

	go s.pollLoop(symbol)
}

// stopPolling stops REST API polling for a symbol.
func (s *MarketDataService) stopPolling(symbol string) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	delete(s.polling, symbol)
}

// pollLoop periodically fetches kline and order book data via REST API.
func (s *MarketDataService) pollLoop(symbol string) {
	ticker := time.NewTicker(restPollInterval)
	defer ticker.Stop()

	s.log.Info("started REST polling fallback", zap.String("symbol", symbol))

	for {
		select {
		case <-s.pollCtx.Done():
			return
		case <-ticker.C:
			// Check if polling is still active for this symbol.
			s.pollMu.Lock()
			active := s.polling[symbol]
			s.pollMu.Unlock()
			if !active {
				return
			}

			// Check if WebSocket reconnected; if so, stop polling.
			if s.ws.IsConnected() {
				s.log.Info("websocket reconnected, stopping REST polling",
					zap.String("symbol", symbol))
				if err := s.setupWebSocketSubscriptions(symbol); err != nil {
					s.log.Warn("failed to re-setup websocket after reconnect",
						zap.String("symbol", symbol), zap.Error(err))
					continue // keep polling
				}
				s.stopPolling(symbol)
				return
			}

			s.pollOnce(symbol)
		}
	}
}

// pollOnce fetches the latest kline and order book data via REST API and
// distributes it to subscribers.
func (s *MarketDataService) pollOnce(symbol string) {
	// Fetch latest 1m kline.
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

	// Fetch order book.
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

// --- helpers ---

// klineCacheKey returns the cache key for a symbol and interval combination.
func klineCacheKey(symbol, interval string) string {
	return symbol + ":" + interval
}

// isValidInterval checks whether the given interval is supported.
func isValidInterval(interval string) bool {
	for _, v := range SupportedIntervals {
		if v == interval {
			return true
		}
	}
	return false
}
