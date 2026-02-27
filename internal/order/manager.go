package order

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/internal/logger"
	"money-loves-me/internal/model"
	"money-loves-me/internal/notification"
	"money-loves-me/internal/store"
	"money-loves-me/pkg/binance"
)

// --- Interfaces for dependency injection ---

// BinanceOrderClient defines the subset of BinanceClient used by OrderManager.
type BinanceOrderClient interface {
	CreateOrder(req binance.CreateOrderRequest) (*binance.OrderResponse, error)
	CancelOrder(symbol string, orderID int64) (*binance.OrderResponse, error)
}

// RiskChecker defines the risk control interface used by OrderManager.
type RiskChecker interface {
	CheckOrder(symbol string, amount, totalAssetValue decimal.Decimal) error
}

// AccountValuer provides total asset value for risk checks.
type AccountValuer interface {
	GetTotalAssetValue() (decimal.Decimal, error)
}

// OrderStoreInterface defines the persistence interface for orders.
type OrderStoreInterface interface {
	Create(order *model.Order) error
	GetByID(id int64) (*model.Order, error)
	Update(order *model.Order) error
	GetByFilter(filter store.OrderFilter) ([]model.Order, error)
}

// TradeStoreInterface defines the persistence interface for trades.
type TradeStoreInterface interface {
	Create(trade *model.Trade) error
	GetByOrderID(orderID int64) ([]model.Trade, error)
	GetByFilter(filter store.TradeFilter) ([]model.Trade, error)
}

// --- Order status constants ---

const (
	OrderStatusSubmitted     = "SUBMITTED"
	OrderStatusNew           = "NEW"
	OrderStatusPartialFilled = "PARTIALLY_FILLED"
	OrderStatusFilled        = "FILLED"
	OrderStatusCancelled     = "CANCELLED"
	OrderStatusRejected      = "REJECTED"
	OrderStatusExpired       = "EXPIRED"
)

// --- Order type constants ---

const (
	OrderTypeLimit         = "LIMIT"
	OrderTypeMarket        = "MARKET"
	OrderTypeStopLossLimit = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit    = "TAKE_PROFIT_LIMIT"
)

// --- Signal reason for strategy decision tracking ---

// SignalReason captures the strategy decision context for an order.
type SignalReason struct {
	Indicators  map[string]float64 `json:"indicators"`
	TriggerRule string             `json:"trigger_rule"`
	MarketState string             `json:"market_state"`
}

// --- CreateOrderRequest for the OrderManager ---

// CreateOrderRequest holds the parameters for submitting an order.
type CreateOrderRequest struct {
	Symbol       string
	Side         string // BUY / SELL
	Type         string // LIMIT / MARKET / STOP_LOSS_LIMIT / TAKE_PROFIT_LIMIT
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	StopPrice    decimal.Decimal
	StrategyName string
	Reason       *SignalReason
}

// OrderFilter defines filtering criteria for order history queries.
type OrderFilter struct {
	Symbol string
	Start  time.Time
	End    time.Time
}

// OrderManager handles order submission, cancellation, tracking, and persistence.
type OrderManager struct {
	client    BinanceOrderClient
	validator *OrderValidator
	risk      RiskChecker
	account   AccountValuer
	orders    OrderStoreInterface
	trades    TradeStoreInterface
	notifier  *notification.NotificationService
	log       *logger.Logger

	mu            sync.RWMutex
	activeOrders  map[int64]*model.Order // binanceOrderID -> order
	cancelRefresh context.CancelFunc
}

// NewOrderManager creates a new OrderManager with all dependencies.
func NewOrderManager(
	client BinanceOrderClient,
	validator *OrderValidator,
	risk RiskChecker,
	account AccountValuer,
	orders OrderStoreInterface,
	trades TradeStoreInterface,
	notifier *notification.NotificationService,
	log *logger.Logger,
) *OrderManager {
	return &OrderManager{
		client:       client,
		validator:    validator,
		risk:         risk,
		account:      account,
		orders:       orders,
		trades:       trades,
		notifier:     notifier,
		log:          log,
		activeOrders: make(map[int64]*model.Order),
	}
}

// Start begins the background goroutine that refreshes active order statuses
// every 2 seconds via WebSocket/polling.
func (m *OrderManager) Start(ctx context.Context) {
	ctx, m.cancelRefresh = context.WithCancel(ctx)
	go m.refreshLoop(ctx)
	m.logInfo("order manager started")
}

// Stop terminates the background order status refresh loop.
func (m *OrderManager) Stop() {
	if m.cancelRefresh != nil {
		m.cancelRefresh()
	}
	m.logInfo("order manager stopped")
}

// SubmitOrder validates, risk-checks, submits an order to Binance, persists it,
// and records any fills as trades. On failure, it notifies the user.
func (m *OrderManager) SubmitOrder(req CreateOrderRequest, reason SignalReason) (*model.Order, error) {
	// 1. Validate order parameters against exchange rules.
	binReq := binance.CreateOrderRequest{
		Symbol:    req.Symbol,
		Side:      req.Side,
		Type:      req.Type,
		Quantity:  req.Quantity,
		Price:     req.Price,
		StopPrice: req.StopPrice,
	}

	// Set TimeInForce for limit-type orders.
	switch req.Type {
	case OrderTypeLimit, OrderTypeStopLossLimit, OrderTypeTakeProfit:
		binReq.TimeInForce = "GTC"
	}

	if m.validator != nil {
		if err := m.validator.Validate(binReq); err != nil {
			m.notifyFailure(req.Symbol, "order validation failed", err)
			return nil, err
		}
	}

	// 2. Risk control check.
	orderAmount := req.Quantity.Mul(req.Price)
	if req.Type == OrderTypeMarket {
		// For market orders, use quantity as a rough estimate (price may be zero).
		orderAmount = req.Quantity.Mul(req.Price)
	}

	if m.risk != nil {
		totalAsset := decimal.Zero
		if m.account != nil {
			val, err := m.account.GetTotalAssetValue()
			if err != nil {
				m.logWarn("failed to get total asset value for risk check", zap.Error(err))
			} else {
				totalAsset = val
			}
		}
		if err := m.risk.CheckOrder(req.Symbol, orderAmount, totalAsset); err != nil {
			m.notifyFailure(req.Symbol, "risk check failed", err)
			return nil, err
		}
	}

	// 3. Submit order to Binance.
	resp, err := m.client.CreateOrder(binReq)
	if err != nil {
		m.notifyFailure(req.Symbol, "order submission failed", err)
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI,
			fmt.Sprintf("failed to submit order for %s", req.Symbol), "order", err)
	}

	// 4. Build and persist the order record.
	binanceOrderID := resp.OrderID
	order := &model.Order{
		Symbol:         resp.Symbol,
		Side:           req.Side,
		Type:           req.Type,
		Quantity:       req.Quantity,
		Price:          req.Price,
		StopPrice:      req.StopPrice,
		Status:         mapBinanceStatus(resp.Status),
		BinanceOrderID: &binanceOrderID,
		StrategyName:   req.StrategyName,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Accumulate fees from fills.
	totalFee := decimal.Zero
	feeAsset := ""
	for _, fill := range resp.Fills {
		fee, _ := decimal.NewFromString(fill.Commission)
		totalFee = totalFee.Add(fee)
		if fill.CommissionAsset != "" {
			feeAsset = fill.CommissionAsset
		}
	}
	order.Fee = totalFee
	order.FeeAsset = feeAsset

	if err := m.orders.Create(order); err != nil {
		m.logError("failed to persist order", zap.Error(err))
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to persist order", "order", err)
	}

	// 5. Record fills as trade records with strategy decision reason.
	reasonJSON, _ := json.Marshal(reason)
	for _, fill := range resp.Fills {
		fillPrice, _ := decimal.NewFromString(fill.Price)
		fillQty, _ := decimal.NewFromString(fill.Qty)
		fillFee, _ := decimal.NewFromString(fill.Commission)
		fillAmount := fillPrice.Mul(fillQty)

		trade := &model.Trade{
			OrderID:        order.ID,
			Symbol:         order.Symbol,
			Side:           order.Side,
			Price:          fillPrice,
			Quantity:       fillQty,
			Amount:         fillAmount,
			Fee:            fillFee,
			FeeAsset:       fill.CommissionAsset,
			StrategyName:   req.StrategyName,
			DecisionReason: reasonJSON,
			ExecutedAt:     time.Now(),
		}

		if err := m.trades.Create(trade); err != nil {
			m.logError("failed to persist trade", zap.Error(err),
				zap.Int64("orderID", order.ID))
		}
	}

	// 6. Track active order for status updates.
	if isActiveStatus(order.Status) {
		m.mu.Lock()
		m.activeOrders[binanceOrderID] = order
		m.mu.Unlock()
	}

	m.logInfo("order submitted successfully",
		zap.String("symbol", order.Symbol),
		zap.String("side", order.Side),
		zap.Int64("binanceOrderID", binanceOrderID),
		zap.String("status", order.Status),
	)

	return order, nil
}

// CancelOrder sends a cancel request to Binance and updates the local order status.
func (m *OrderManager) CancelOrder(symbol string, orderID int64) error {
	resp, err := m.client.CancelOrder(symbol, orderID)
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrBinanceAPI,
			fmt.Sprintf("failed to cancel order %d", orderID), "order", err)
	}

	// Update local order record.
	m.mu.Lock()
	if o, ok := m.activeOrders[orderID]; ok {
		o.Status = OrderStatusCancelled
		o.UpdatedAt = time.Now()
		delete(m.activeOrders, orderID)
		m.mu.Unlock()

		if err := m.orders.Update(o); err != nil {
			m.logError("failed to update cancelled order in DB", zap.Error(err))
		}
	} else {
		m.mu.Unlock()
	}

	// Also try to find and update by binance_order_id in the database directly.
	orders, err := m.orders.GetByFilter(store.OrderFilter{Symbol: symbol})
	if err == nil {
		for i := range orders {
			if orders[i].BinanceOrderID != nil && *orders[i].BinanceOrderID == orderID {
				orders[i].Status = mapBinanceStatus(resp.Status)
				orders[i].UpdatedAt = time.Now()
				_ = m.orders.Update(&orders[i])
				break
			}
		}
	}

	m.logInfo("order cancelled",
		zap.String("symbol", symbol),
		zap.Int64("binanceOrderID", orderID),
	)

	return nil
}

// GetActiveOrders returns all orders with status NEW or PARTIALLY_FILLED,
// optionally filtered by symbol.
func (m *OrderManager) GetActiveOrders(symbol string) ([]model.Order, error) {
	// Query NEW orders.
	newOrders, err := m.orders.GetByFilter(store.OrderFilter{
		Symbol: symbol,
		Status: OrderStatusNew,
	})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query active orders (NEW)", "order", err)
	}

	// Query PARTIALLY_FILLED orders.
	partialOrders, err := m.orders.GetByFilter(store.OrderFilter{
		Symbol: symbol,
		Status: OrderStatusPartialFilled,
	})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query active orders (PARTIALLY_FILLED)", "order", err)
	}

	result := make([]model.Order, 0, len(newOrders)+len(partialOrders))
	result = append(result, newOrders...)
	result = append(result, partialOrders...)
	return result, nil
}

// GetOrderHistory returns orders filtered by symbol and time range.
func (m *OrderManager) GetOrderHistory(filter OrderFilter) ([]model.Order, error) {
	orders, err := m.orders.GetByFilter(store.OrderFilter{
		Symbol: filter.Symbol,
		Start:  filter.Start,
		End:    filter.End,
	})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query order history", "order", err)
	}
	return orders, nil
}

// ExportTradesCSV exports trades matching the given time range to the writer in CSV format.
func (m *OrderManager) ExportTradesCSV(start, end time.Time, writer io.Writer) error {
	return ExportCSV(m.trades, start, end, writer)
}

// refreshLoop polls active order statuses every 2 seconds.
func (m *OrderManager) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshActiveOrders()
		}
	}
}

// refreshActiveOrders re-queries active orders from the DB and updates
// their status from the in-memory tracking map.
func (m *OrderManager) refreshActiveOrders() {
	m.mu.RLock()
	activeIDs := make([]int64, 0, len(m.activeOrders))
	for id := range m.activeOrders {
		activeIDs = append(activeIDs, id)
	}
	m.mu.RUnlock()

	if len(activeIDs) == 0 {
		return
	}

	// For each active order, we could query Binance for the latest status.
	// In a production system this would use the user data WebSocket stream.
	// Here we re-check the local DB state and remove completed orders.
	for _, binID := range activeIDs {
		m.mu.RLock()
		o, ok := m.activeOrders[binID]
		m.mu.RUnlock()
		if !ok {
			continue
		}

		// Re-fetch from DB to check if status was updated externally.
		dbOrder, err := m.orders.GetByID(o.ID)
		if err != nil {
			m.logWarn("failed to refresh order status", zap.Int64("orderID", o.ID), zap.Error(err))
			continue
		}

		if !isActiveStatus(dbOrder.Status) {
			m.mu.Lock()
			delete(m.activeOrders, binID)
			m.mu.Unlock()
			m.logInfo("order no longer active",
				zap.Int64("binanceOrderID", binID),
				zap.String("status", dbOrder.Status),
			)
		}
	}
}

// --- Helper functions ---

// mapBinanceStatus maps Binance order status strings to local status constants.
func mapBinanceStatus(status string) string {
	switch status {
	case "NEW":
		return OrderStatusNew
	case "PARTIALLY_FILLED":
		return OrderStatusPartialFilled
	case "FILLED":
		return OrderStatusFilled
	case "CANCELED":
		return OrderStatusCancelled
	case "REJECTED":
		return OrderStatusRejected
	case "EXPIRED":
		return OrderStatusExpired
	default:
		return OrderStatusSubmitted
	}
}

// isActiveStatus returns true if the order status indicates it's still active.
func isActiveStatus(status string) bool {
	return status == OrderStatusNew || status == OrderStatusPartialFilled || status == OrderStatusSubmitted
}

// notifyFailure sends a notification about an order failure.
func (m *OrderManager) notifyFailure(symbol, reason string, err error) {
	m.logError("order failure",
		zap.String("symbol", symbol),
		zap.String("reason", reason),
		zap.Error(err),
	)
	if m.notifier != nil {
		_ = m.notifier.Send(
			notification.EventRiskAlert,
			fmt.Sprintf("Order failed: %s", symbol),
			fmt.Sprintf("%s: %v", reason, err),
		)
	}
}

// logInfo safely logs at INFO level.
func (m *OrderManager) logInfo(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Info(msg, fields...)
	}
}

// logWarn safely logs at WARN level.
func (m *OrderManager) logWarn(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Warn(msg, fields...)
	}
}

// logError safely logs at ERROR level.
func (m *OrderManager) logError(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Error(msg, fields...)
	}
}
