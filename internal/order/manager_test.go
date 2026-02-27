package order

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"money-loves-me/internal/model"
	"money-loves-me/internal/store"
	"money-loves-me/pkg/binance"
)

// --- Mock implementations ---

// mockBinanceClient implements BinanceOrderClient for testing.
type mockBinanceClient struct {
	createResp *binance.OrderResponse
	createErr  error
	cancelResp *binance.OrderResponse
	cancelErr  error
}

func (m *mockBinanceClient) CreateOrder(req binance.CreateOrderRequest) (*binance.OrderResponse, error) {
	return m.createResp, m.createErr
}

func (m *mockBinanceClient) CancelOrder(symbol string, orderID int64) (*binance.OrderResponse, error) {
	return m.cancelResp, m.cancelErr
}

// mockRiskChecker implements RiskChecker for testing.
type mockRiskChecker struct {
	err error
}

func (m *mockRiskChecker) CheckOrder(symbol string, amount, totalAssetValue decimal.Decimal) error {
	return m.err
}

// mockAccountValuer implements AccountValuer for testing.
type mockAccountValuer struct {
	value decimal.Decimal
	err   error
}

func (m *mockAccountValuer) GetTotalAssetValue() (decimal.Decimal, error) {
	return m.value, m.err
}

// mockOrderStore implements OrderStoreInterface for testing.
type mockOrderStore struct {
	orders  []model.Order
	nextID  int64
	updated []model.Order
}

func newMockOrderStore() *mockOrderStore {
	return &mockOrderStore{nextID: 1}
}

func (m *mockOrderStore) Create(order *model.Order) error {
	order.ID = m.nextID
	m.nextID++
	m.orders = append(m.orders, *order)
	return nil
}

func (m *mockOrderStore) GetByID(id int64) (*model.Order, error) {
	for i := range m.orders {
		if m.orders[i].ID == id {
			return &m.orders[i], nil
		}
	}
	return nil, nil
}

func (m *mockOrderStore) Update(order *model.Order) error {
	for i := range m.orders {
		if m.orders[i].ID == order.ID {
			m.orders[i] = *order
			m.updated = append(m.updated, *order)
			return nil
		}
	}
	m.updated = append(m.updated, *order)
	return nil
}

func (m *mockOrderStore) GetByFilter(filter store.OrderFilter) ([]model.Order, error) {
	var result []model.Order
	for _, o := range m.orders {
		if filter.Symbol != "" && o.Symbol != filter.Symbol {
			continue
		}
		if filter.Status != "" && o.Status != filter.Status {
			continue
		}
		if !filter.Start.IsZero() && o.CreatedAt.Before(filter.Start) {
			continue
		}
		if !filter.End.IsZero() && o.CreatedAt.After(filter.End) {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

// mockTradeStore implements TradeStoreInterface for testing.
type mockTradeStore struct {
	trades []model.Trade
	nextID int64
}

func newMockTradeStore() *mockTradeStore {
	return &mockTradeStore{nextID: 1}
}

func (m *mockTradeStore) Create(trade *model.Trade) error {
	trade.ID = m.nextID
	m.nextID++
	m.trades = append(m.trades, *trade)
	return nil
}

func (m *mockTradeStore) GetByOrderID(orderID int64) ([]model.Trade, error) {
	var result []model.Trade
	for _, t := range m.trades {
		if t.OrderID == orderID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTradeStore) GetByFilter(filter store.TradeFilter) ([]model.Trade, error) {
	var result []model.Trade
	for _, t := range m.trades {
		if filter.Symbol != "" && t.Symbol != filter.Symbol {
			continue
		}
		if filter.StrategyName != "" && t.StrategyName != filter.StrategyName {
			continue
		}
		result = append(result, t)
	}
	return result, nil
}

// --- Helper to build a standard test OrderManager ---

func newTestOrderManager(
	client BinanceOrderClient,
	risk RiskChecker,
	account AccountValuer,
) (*OrderManager, *mockOrderStore, *mockTradeStore) {
	orderStore := newMockOrderStore()
	tradeStore := newMockTradeStore()
	om := NewOrderManager(client, nil, risk, account, orderStore, tradeStore, nil, nil)
	return om, orderStore, tradeStore
}

// --- Tests ---

func TestSubmitOrder_Success(t *testing.T) {
	binanceID := int64(12345)
	client := &mockBinanceClient{
		createResp: &binance.OrderResponse{
			Symbol:  "BTCUSDT",
			OrderID: binanceID,
			Status:  "NEW",
			Fills: []binance.OrderFill{
				{Price: "50000.00", Qty: "0.1", Commission: "0.05", CommissionAsset: "BNB"},
			},
		},
	}
	risk := &mockRiskChecker{}
	account := &mockAccountValuer{value: decimal.NewFromInt(100000)}

	om, orderStore, tradeStore := newTestOrderManager(client, risk, account)

	req := CreateOrderRequest{
		Symbol:       "BTCUSDT",
		Side:         "BUY",
		Type:         OrderTypeLimit,
		Quantity:     decimal.NewFromFloat(0.1),
		Price:        decimal.NewFromFloat(50000),
		StrategyName: "ma_cross",
	}
	reason := SignalReason{
		Indicators:  map[string]float64{"MA7": 50100, "MA25": 49800},
		TriggerRule: "MA7 crossed above MA25",
		MarketState: "uptrend",
	}

	order, err := om.SubmitOrder(req, reason)
	require.NoError(t, err)
	require.NotNil(t, order)

	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, "BUY", order.Side)
	assert.Equal(t, OrderTypeLimit, order.Type)
	assert.Equal(t, OrderStatusNew, order.Status)
	assert.Equal(t, &binanceID, order.BinanceOrderID)
	assert.Equal(t, "ma_cross", order.StrategyName)
	assert.True(t, order.Fee.GreaterThan(decimal.Zero))

	// Verify order was persisted.
	assert.Len(t, orderStore.orders, 1)

	// Verify trade was persisted with decision reason.
	assert.Len(t, tradeStore.trades, 1)
	trade := tradeStore.trades[0]
	assert.Equal(t, order.ID, trade.OrderID)
	assert.Equal(t, "BTCUSDT", trade.Symbol)
	assert.Equal(t, "BUY", trade.Side)
	assert.Equal(t, "BNB", trade.FeeAsset)
	assert.Equal(t, "ma_cross", trade.StrategyName)

	// Verify decision reason JSON.
	var dr model.DecisionReasonJSON
	err = json.Unmarshal(trade.DecisionReason, &dr)
	require.NoError(t, err)
	assert.Equal(t, "MA7 crossed above MA25", dr.TriggerRule)
	assert.Equal(t, "uptrend", dr.MarketState)

	// Verify active order tracking.
	om.mu.RLock()
	_, tracked := om.activeOrders[binanceID]
	om.mu.RUnlock()
	assert.True(t, tracked)
}

func TestSubmitOrder_RiskCheckFails(t *testing.T) {
	client := &mockBinanceClient{
		createResp: &binance.OrderResponse{},
	}
	risk := &mockRiskChecker{
		err: assert.AnError,
	}
	account := &mockAccountValuer{value: decimal.NewFromInt(100000)}

	om, orderStore, _ := newTestOrderManager(client, risk, account)

	req := CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     OrderTypeMarket,
		Quantity: decimal.NewFromFloat(1),
		Price:    decimal.NewFromFloat(50000),
	}

	order, err := om.SubmitOrder(req, SignalReason{})
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Len(t, orderStore.orders, 0) // No order should be persisted.
}

func TestSubmitOrder_BinanceError(t *testing.T) {
	client := &mockBinanceClient{
		createErr: assert.AnError,
	}
	risk := &mockRiskChecker{}
	account := &mockAccountValuer{value: decimal.NewFromInt(100000)}

	om, orderStore, _ := newTestOrderManager(client, risk, account)

	req := CreateOrderRequest{
		Symbol:   "ETHUSDT",
		Side:     "SELL",
		Type:     OrderTypeMarket,
		Quantity: decimal.NewFromFloat(5),
		Price:    decimal.NewFromFloat(3000),
	}

	order, err := om.SubmitOrder(req, SignalReason{})
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Len(t, orderStore.orders, 0)
}

func TestCancelOrder_Success(t *testing.T) {
	binanceID := int64(99999)
	client := &mockBinanceClient{
		cancelResp: &binance.OrderResponse{
			Symbol:  "BTCUSDT",
			OrderID: binanceID,
			Status:  "CANCELED",
		},
	}

	om, orderStore, _ := newTestOrderManager(client, nil, nil)

	// Pre-populate an active order.
	order := &model.Order{
		Symbol:         "BTCUSDT",
		Side:           "BUY",
		Type:           OrderTypeLimit,
		Quantity:       decimal.NewFromFloat(0.5),
		Price:          decimal.NewFromFloat(45000),
		Status:         OrderStatusNew,
		BinanceOrderID: &binanceID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	_ = orderStore.Create(order)

	om.mu.Lock()
	om.activeOrders[binanceID] = order
	om.mu.Unlock()

	err := om.CancelOrder("BTCUSDT", binanceID)
	require.NoError(t, err)

	// Verify order removed from active tracking.
	om.mu.RLock()
	_, tracked := om.activeOrders[binanceID]
	om.mu.RUnlock()
	assert.False(t, tracked)
}

func TestCancelOrder_BinanceError(t *testing.T) {
	client := &mockBinanceClient{
		cancelErr: assert.AnError,
	}

	om, _, _ := newTestOrderManager(client, nil, nil)

	err := om.CancelOrder("BTCUSDT", 12345)
	assert.Error(t, err)
}

func TestGetActiveOrders(t *testing.T) {
	client := &mockBinanceClient{}
	om, orderStore, _ := newTestOrderManager(client, nil, nil)

	now := time.Now()
	binID1 := int64(1)
	binID2 := int64(2)
	binID3 := int64(3)

	// Create orders with different statuses.
	_ = orderStore.Create(&model.Order{
		Symbol: "BTCUSDT", Status: OrderStatusNew,
		BinanceOrderID: &binID1, CreatedAt: now,
	})
	_ = orderStore.Create(&model.Order{
		Symbol: "BTCUSDT", Status: OrderStatusPartialFilled,
		BinanceOrderID: &binID2, CreatedAt: now,
	})
	_ = orderStore.Create(&model.Order{
		Symbol: "BTCUSDT", Status: OrderStatusFilled,
		BinanceOrderID: &binID3, CreatedAt: now,
	})

	active, err := om.GetActiveOrders("BTCUSDT")
	require.NoError(t, err)
	assert.Len(t, active, 2) // Only NEW and PARTIALLY_FILLED.
}

func TestGetOrderHistory_FilterBySymbolAndTime(t *testing.T) {
	client := &mockBinanceClient{}
	om, orderStore, _ := newTestOrderManager(client, nil, nil)

	now := time.Now()
	binID1 := int64(1)
	binID2 := int64(2)

	_ = orderStore.Create(&model.Order{
		Symbol: "BTCUSDT", Status: OrderStatusFilled,
		BinanceOrderID: &binID1, CreatedAt: now.Add(-1 * time.Hour),
	})
	_ = orderStore.Create(&model.Order{
		Symbol: "ETHUSDT", Status: OrderStatusFilled,
		BinanceOrderID: &binID2, CreatedAt: now,
	})

	// Filter by symbol.
	orders, err := om.GetOrderHistory(OrderFilter{Symbol: "BTCUSDT"})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, "BTCUSDT", orders[0].Symbol)

	// Filter by time range.
	orders, err = om.GetOrderHistory(OrderFilter{
		Start: now.Add(-30 * time.Minute),
		End:   now.Add(1 * time.Minute),
	})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, "ETHUSDT", orders[0].Symbol)
}

func TestSubmitOrder_AllOrderTypes(t *testing.T) {
	orderTypes := []string{
		OrderTypeLimit,
		OrderTypeMarket,
		OrderTypeStopLossLimit,
		OrderTypeTakeProfit,
	}

	for _, ot := range orderTypes {
		t.Run(ot, func(t *testing.T) {
			binanceID := int64(100)
			client := &mockBinanceClient{
				createResp: &binance.OrderResponse{
					Symbol:  "BTCUSDT",
					OrderID: binanceID,
					Status:  "NEW",
				},
			}

			om, orderStore, _ := newTestOrderManager(client, nil, nil)

			req := CreateOrderRequest{
				Symbol:    "BTCUSDT",
				Side:      "BUY",
				Type:      ot,
				Quantity:  decimal.NewFromFloat(0.1),
				Price:     decimal.NewFromFloat(50000),
				StopPrice: decimal.NewFromFloat(49000),
			}

			order, err := om.SubmitOrder(req, SignalReason{})
			require.NoError(t, err)
			require.NotNil(t, order)
			assert.Equal(t, ot, order.Type)
			assert.Len(t, orderStore.orders, 1)
		})
	}
}

func TestMapBinanceStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"NEW", OrderStatusNew},
		{"PARTIALLY_FILLED", OrderStatusPartialFilled},
		{"FILLED", OrderStatusFilled},
		{"CANCELED", OrderStatusCancelled},
		{"REJECTED", OrderStatusRejected},
		{"EXPIRED", OrderStatusExpired},
		{"UNKNOWN", OrderStatusSubmitted},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapBinanceStatus(tt.input))
		})
	}
}

func TestIsActiveStatus(t *testing.T) {
	assert.True(t, isActiveStatus(OrderStatusNew))
	assert.True(t, isActiveStatus(OrderStatusPartialFilled))
	assert.True(t, isActiveStatus(OrderStatusSubmitted))
	assert.False(t, isActiveStatus(OrderStatusFilled))
	assert.False(t, isActiveStatus(OrderStatusCancelled))
	assert.False(t, isActiveStatus(OrderStatusRejected))
	assert.False(t, isActiveStatus(OrderStatusExpired))
}
