package market

import (
	"sync"
	"testing"
	"time"

	"money-loves-me/pkg/binance"

	"github.com/shopspring/decimal"
)

// mockConsumer 实现 DataConsumer 接口，用于测试。
type mockConsumer struct {
	mu           sync.Mutex
	klineUpdates []klineUpdate
	bookUpdates  []bookUpdate
}

type klineUpdate struct {
	symbol string
	kline  binance.Kline
}

type bookUpdate struct {
	symbol string
	book   *binance.OrderBook
}

func (m *mockConsumer) OnKlineUpdate(symbol string, kline binance.Kline) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.klineUpdates = append(m.klineUpdates, klineUpdate{symbol: symbol, kline: kline})
}

func (m *mockConsumer) OnOrderBookUpdate(symbol string, book *binance.OrderBook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bookUpdates = append(m.bookUpdates, bookUpdate{symbol: symbol, book: book})
}

func (m *mockConsumer) getKlineCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.klineUpdates)
}

func (m *mockConsumer) getBookCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.bookUpdates)
}

func TestIsValidInterval(t *testing.T) {
	valid := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
	for _, v := range valid {
		if !isValidInterval(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}

	invalid := []string{"2m", "3h", "1w", "", "1M"}
	for _, v := range invalid {
		if isValidInterval(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestKlineCacheKey(t *testing.T) {
	key := klineCacheKey("BTCUSDT", "1m")
	if key != "BTCUSDT:1m" {
		t.Errorf("expected BTCUSDT:1m, got %s", key)
	}
}

func TestSubscribeNilConsumer(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	err := svc.Subscribe("BTCUSDT", nil)
	if err == nil {
		t.Error("expected error for nil consumer")
	}
}

func TestSubscribeEmptySymbol(t *testing.T) {
	consumer := &mockConsumer{}
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	err := svc.Subscribe("", consumer)
	if err == nil {
		t.Error("expected error for empty symbol")
	}
}

func TestUnsubscribeNilConsumer(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	err := svc.Unsubscribe("BTCUSDT", nil)
	if err == nil {
		t.Error("expected error for nil consumer")
	}
}

func TestHandleKlineEvent_UpdatesCache(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	consumer := &mockConsumer{}
	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{consumer}
	svc.mu.Unlock()

	now := time.Now()
	event := &binance.WsKlineEvent{
		Symbol:   "BTCUSDT",
		Interval: "1m",
		Kline: binance.Kline{
			OpenTime:  now,
			Open:      decimal.NewFromFloat(42000),
			High:      decimal.NewFromFloat(42500),
			Low:       decimal.NewFromFloat(41800),
			Close:     decimal.NewFromFloat(42300),
			Volume:    decimal.NewFromFloat(100),
			CloseTime: now.Add(time.Minute),
		},
	}

	svc.handleKlineEvent(event)

	// 验证缓存已更新。
	svc.mu.RLock()
	cached := svc.klineCache["BTCUSDT:1m"]
	svc.mu.RUnlock()

	if len(cached) != 1 {
		t.Fatalf("expected 1 cached kline, got %d", len(cached))
	}
	if !cached[0].Close.Equal(decimal.NewFromFloat(42300)) {
		t.Errorf("expected close 42300, got %s", cached[0].Close)
	}

	// 验证消费者已收到通知。
	if consumer.getKlineCount() != 1 {
		t.Errorf("expected 1 kline update, got %d", consumer.getKlineCount())
	}
}

func TestHandleKlineEvent_UpdatesExistingKline(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	now := time.Now()
	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{&mockConsumer{}}
	svc.klineCache["BTCUSDT:1m"] = []binance.Kline{
		{OpenTime: now, Close: decimal.NewFromFloat(42000), CloseTime: now.Add(time.Minute)},
	}
	svc.mu.Unlock()

	// 相同的 OpenTime 应该原地更新。
	event := &binance.WsKlineEvent{
		Symbol:   "BTCUSDT",
		Interval: "1m",
		Kline: binance.Kline{
			OpenTime:  now,
			Close:     decimal.NewFromFloat(42500),
			CloseTime: now.Add(time.Minute),
		},
	}

	svc.handleKlineEvent(event)

	svc.mu.RLock()
	cached := svc.klineCache["BTCUSDT:1m"]
	svc.mu.RUnlock()

	if len(cached) != 1 {
		t.Fatalf("expected 1 cached kline (updated in place), got %d", len(cached))
	}
	if !cached[0].Close.Equal(decimal.NewFromFloat(42500)) {
		t.Errorf("expected updated close 42500, got %s", cached[0].Close)
	}
}

func TestHandleKlineEvent_NilEvent(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	// 不应该 panic。
	svc.handleKlineEvent(nil)
}

func TestHandleOrderBookEvent_UpdatesCache(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	consumer := &mockConsumer{}
	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{consumer}
	svc.mu.Unlock()

	event := &binance.WsOrderBookEvent{
		Symbol: "BTCUSDT",
		Book: binance.OrderBook{
			Symbol: "BTCUSDT",
			Bids: []binance.PriceLevel{
				{Price: decimal.NewFromFloat(42000), Quantity: decimal.NewFromFloat(1.5)},
			},
			Asks: []binance.PriceLevel{
				{Price: decimal.NewFromFloat(42100), Quantity: decimal.NewFromFloat(2.0)},
			},
			UpdateTime: time.Now(),
		},
	}

	svc.handleOrderBookEvent(event)

	svc.mu.RLock()
	book := svc.orderBooks["BTCUSDT"]
	svc.mu.RUnlock()

	if book == nil {
		t.Fatal("expected order book to be cached")
	}
	if len(book.Bids) != 1 || !book.Bids[0].Price.Equal(decimal.NewFromFloat(42000)) {
		t.Error("unexpected bid data")
	}

	if consumer.getBookCount() != 1 {
		t.Errorf("expected 1 book update, got %d", consumer.getBookCount())
	}
}

func TestHandleOrderBookEvent_NilEvent(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	// 不应该 panic。
	svc.handleOrderBookEvent(nil)
}

func TestGetCurrentPrice_FromKlineCache(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	now := time.Now()
	svc.mu.Lock()
	svc.klineCache["BTCUSDT:1m"] = []binance.Kline{
		{
			OpenTime:  now.Add(-time.Minute),
			Close:     decimal.NewFromFloat(42300),
			CloseTime: now,
		},
	}
	svc.mu.Unlock()

	price, err := svc.GetCurrentPrice("BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !price.Equal(decimal.NewFromFloat(42300)) {
		t.Errorf("expected 42300, got %s", price)
	}
}

func TestGetCurrentPrice_FromOrderBook(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	svc.mu.Lock()
	svc.orderBooks["BTCUSDT"] = &binance.OrderBook{
		Bids: []binance.PriceLevel{{Price: decimal.NewFromFloat(42000), Quantity: decimal.NewFromFloat(1)}},
		Asks: []binance.PriceLevel{{Price: decimal.NewFromFloat(42100), Quantity: decimal.NewFromFloat(1)}},
	}
	svc.mu.Unlock()

	price, err := svc.GetCurrentPrice("BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 中间价 = (42000 + 42100) / 2 = 42050
	expected := decimal.NewFromFloat(42050)
	if !price.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, price)
	}
}

func TestGetCurrentPrice_NoData(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	_, err := svc.GetCurrentPrice("BTCUSDT")
	if err == nil {
		t.Error("expected error when no data available")
	}
}

func TestGetHistoricalKlines_InvalidInterval(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	_, err := svc.GetHistoricalKlines("BTCUSDT", "2m", time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Error("expected error for invalid interval")
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	consumer := &mockConsumer{}
	// 手动添加订阅者以避免 WebSocket 设置。
	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{consumer}
	svc.mu.Unlock()

	// 使用相同消费者的第二次订阅应该是空操作。
	err := svc.Subscribe("BTCUSDT", consumer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.mu.RLock()
	count := len(svc.subscribers["BTCUSDT"])
	svc.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 subscriber (no duplicate), got %d", count)
	}
}

func TestUnsubscribe_RemovesConsumer(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	c1 := &mockConsumer{}
	c2 := &mockConsumer{}

	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{c1, c2}
	svc.mu.Unlock()

	err := svc.Unsubscribe("BTCUSDT", c1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.mu.RLock()
	subs := svc.subscribers["BTCUSDT"]
	svc.mu.RUnlock()

	if len(subs) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}
	if subs[0] != c2 {
		t.Error("expected c2 to remain")
	}
}

func TestUnsubscribe_LastConsumer_CleansUp(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	consumer := &mockConsumer{}
	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{consumer}
	svc.mu.Unlock()

	err := svc.Unsubscribe("BTCUSDT", consumer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.mu.RLock()
	_, exists := svc.subscribers["BTCUSDT"]
	svc.mu.RUnlock()

	if exists {
		t.Error("expected symbol to be removed from subscribers map")
	}
}

func TestGetOrderBook_FromCache(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	expected := &binance.OrderBook{
		Symbol: "ETHUSDT",
		Bids:   []binance.PriceLevel{{Price: decimal.NewFromFloat(3000), Quantity: decimal.NewFromFloat(10)}},
		Asks:   []binance.PriceLevel{{Price: decimal.NewFromFloat(3001), Quantity: decimal.NewFromFloat(5)}},
	}

	svc.mu.Lock()
	svc.orderBooks["ETHUSDT"] = expected
	svc.mu.Unlock()

	book, err := svc.GetOrderBook("ETHUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book.Symbol != "ETHUSDT" {
		t.Errorf("expected ETHUSDT, got %s", book.Symbol)
	}
	if len(book.Bids) != 1 {
		t.Errorf("expected 1 bid, got %d", len(book.Bids))
	}
}

func TestKlineCacheBounded(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{&mockConsumer{}}
	svc.mu.Unlock()

	// 通过事件添加 1001 条K线。
	base := time.Now().Add(-1001 * time.Minute)
	for i := 0; i < 1001; i++ {
		event := &binance.WsKlineEvent{
			Symbol:   "BTCUSDT",
			Interval: "1m",
			Kline: binance.Kline{
				OpenTime:  base.Add(time.Duration(i) * time.Minute),
				Close:     decimal.NewFromInt(int64(i)),
				CloseTime: base.Add(time.Duration(i+1) * time.Minute),
			},
		}
		svc.handleKlineEvent(event)
	}

	svc.mu.RLock()
	cached := svc.klineCache["BTCUSDT:1m"]
	svc.mu.RUnlock()

	if len(cached) > 1000 {
		t.Errorf("cache should be bounded to 1000, got %d", len(cached))
	}
}

func TestMultipleConsumers_AllNotified(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	c1 := &mockConsumer{}
	c2 := &mockConsumer{}
	c3 := &mockConsumer{}

	svc.mu.Lock()
	svc.subscribers["BTCUSDT"] = []DataConsumer{c1, c2, c3}
	svc.mu.Unlock()

	event := &binance.WsKlineEvent{
		Symbol:   "BTCUSDT",
		Interval: "1m",
		Kline: binance.Kline{
			OpenTime:  time.Now(),
			Close:     decimal.NewFromFloat(42000),
			CloseTime: time.Now().Add(time.Minute),
		},
	}

	svc.handleKlineEvent(event)

	if c1.getKlineCount() != 1 || c2.getKlineCount() != 1 || c3.getKlineCount() != 1 {
		t.Error("all consumers should have received the kline update")
	}
}

func TestGetCurrentPrice_PicksMostRecent(t *testing.T) {
	svc := NewMarketDataService(nil, nil, nil)
	defer svc.Close()

	now := time.Now()
	svc.mu.Lock()
	// 1分钟缓存包含较旧的数据。
	svc.klineCache["BTCUSDT:1m"] = []binance.Kline{
		{OpenTime: now.Add(-2 * time.Minute), Close: decimal.NewFromFloat(41000), CloseTime: now.Add(-time.Minute)},
	}
	// 5分钟缓存包含较新的数据。
	svc.klineCache["BTCUSDT:5m"] = []binance.Kline{
		{OpenTime: now.Add(-5 * time.Minute), Close: decimal.NewFromFloat(42000), CloseTime: now},
	}
	svc.mu.Unlock()

	price, err := svc.GetCurrentPrice("BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应该选择5分钟的价格，因为其 CloseTime 更近。
	if !price.Equal(decimal.NewFromFloat(42000)) {
		t.Errorf("expected 42000 (most recent), got %s", price)
	}
}
