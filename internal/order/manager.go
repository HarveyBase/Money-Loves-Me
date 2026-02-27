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

// --- 依赖注入接口 ---

// BinanceOrderClient 定义了 OrderManager 使用的 BinanceClient 子集。
type BinanceOrderClient interface {
	CreateOrder(req binance.CreateOrderRequest) (*binance.OrderResponse, error)
	CancelOrder(symbol string, orderID int64) (*binance.OrderResponse, error)
}

// RiskChecker 定义了 OrderManager 使用的风控接口。
type RiskChecker interface {
	CheckOrder(symbol string, amount, totalAssetValue decimal.Decimal) error
}

// AccountValuer 提供用于风控检查的总资产价值。
type AccountValuer interface {
	GetTotalAssetValue() (decimal.Decimal, error)
}

// OrderStoreInterface 定义了订单的持久化接口。
type OrderStoreInterface interface {
	Create(order *model.Order) error
	GetByID(id int64) (*model.Order, error)
	Update(order *model.Order) error
	GetByFilter(filter store.OrderFilter) ([]model.Order, error)
}

// TradeStoreInterface 定义了交易记录的持久化接口。
type TradeStoreInterface interface {
	Create(trade *model.Trade) error
	GetByOrderID(orderID int64) ([]model.Trade, error)
	GetByFilter(filter store.TradeFilter) ([]model.Trade, error)
}

// --- 订单状态常量 ---

const (
	OrderStatusSubmitted     = "SUBMITTED"
	OrderStatusNew           = "NEW"
	OrderStatusPartialFilled = "PARTIALLY_FILLED"
	OrderStatusFilled        = "FILLED"
	OrderStatusCancelled     = "CANCELLED"
	OrderStatusRejected      = "REJECTED"
	OrderStatusExpired       = "EXPIRED"
)

// --- 订单类型常量 ---

const (
	OrderTypeLimit         = "LIMIT"
	OrderTypeMarket        = "MARKET"
	OrderTypeStopLossLimit = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit    = "TAKE_PROFIT_LIMIT"
)

// --- 策略决策跟踪的信号原因 ---

// SignalReason 记录订单的策略决策上下文。
type SignalReason struct {
	Indicators  map[string]float64 `json:"indicators"`
	TriggerRule string             `json:"trigger_rule"`
	MarketState string             `json:"market_state"`
}

// --- OrderManager 的创建订单请求 ---

// CreateOrderRequest 保存提交订单所需的参数。
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

// OrderFilter 定义订单历史查询的过滤条件。
type OrderFilter struct {
	Symbol string
	Start  time.Time
	End    time.Time
}

// OrderManager 处理订单提交、取消、跟踪和持久化。
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
	activeOrders  map[int64]*model.Order // binanceOrderID -> 订单
	cancelRefresh context.CancelFunc
}

// NewOrderManager 使用所有依赖项创建一个新的 OrderManager。
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

// Start 启动后台协程，每 2 秒通过 WebSocket/轮询刷新活跃订单状态。
func (m *OrderManager) Start(ctx context.Context) {
	ctx, m.cancelRefresh = context.WithCancel(ctx)
	go m.refreshLoop(ctx)
	m.logInfo("order manager started")
}

// Stop 终止后台订单状态刷新循环。
func (m *OrderManager) Stop() {
	if m.cancelRefresh != nil {
		m.cancelRefresh()
	}
	m.logInfo("order manager stopped")
}

// SubmitOrder 验证、风控检查、向 Binance 提交订单、持久化订单，
// 并将所有成交记录为交易。失败时通知用户。
func (m *OrderManager) SubmitOrder(req CreateOrderRequest, reason SignalReason) (*model.Order, error) {
	// 1. 根据交易所规则验证订单参数。
	binReq := binance.CreateOrderRequest{
		Symbol:    req.Symbol,
		Side:      req.Side,
		Type:      req.Type,
		Quantity:  req.Quantity,
		Price:     req.Price,
		StopPrice: req.StopPrice,
	}

	// 为限价类型订单设置 TimeInForce。
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

	// 2. 风控检查。
	orderAmount := req.Quantity.Mul(req.Price)
	if req.Type == OrderTypeMarket {
		// 对于市价单，使用数量作为粗略估算（价格可能为零）。
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

	// 3. 向 Binance 提交订单。
	resp, err := m.client.CreateOrder(binReq)
	if err != nil {
		m.notifyFailure(req.Symbol, "order submission failed", err)
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI,
			fmt.Sprintf("failed to submit order for %s", req.Symbol), "order", err)
	}

	// 4. 构建并持久化订单记录。
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

	// 累计成交手续费。
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

	// 5. 将成交记录为交易记录，包含策略决策原因。
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

	// 6. 跟踪活跃订单以进行状态更新。
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

// CancelOrder 向 Binance 发送取消请求并更新本地订单状态。
func (m *OrderManager) CancelOrder(symbol string, orderID int64) error {
	resp, err := m.client.CancelOrder(symbol, orderID)
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrBinanceAPI,
			fmt.Sprintf("failed to cancel order %d", orderID), "order", err)
	}

	// 更新本地订单记录。
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

	// 同时尝试通过 binance_order_id 直接在数据库中查找并更新。
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

// GetActiveOrders 返回所有状态为 NEW 或 PARTIALLY_FILLED 的订单，
// 可选按交易对过滤。
func (m *OrderManager) GetActiveOrders(symbol string) ([]model.Order, error) {
	// 查询 NEW 状态的订单。
	newOrders, err := m.orders.GetByFilter(store.OrderFilter{
		Symbol: symbol,
		Status: OrderStatusNew,
	})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query active orders (NEW)", "order", err)
	}

	// 查询 PARTIALLY_FILLED 状态的订单。
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

// GetOrderHistory 返回按交易对和时间范围过滤的订单。
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

// ExportTradesCSV 将匹配给定时间范围的交易记录以 CSV 格式导出到 writer。
func (m *OrderManager) ExportTradesCSV(start, end time.Time, writer io.Writer) error {
	return ExportCSV(m.trades, start, end, writer)
}

// refreshLoop 每 2 秒轮询活跃订单状态。
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

// refreshActiveOrders 从数据库重新查询活跃订单，
// 并根据内存跟踪映射更新其状态。
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

	// 对于每个活跃订单，可以向 Binance 查询最新状态。
	// 在生产系统中，这将使用用户数据 WebSocket 流。
	// 这里我们重新检查本地数据库状态并移除已完成的订单。
	for _, binID := range activeIDs {
		m.mu.RLock()
		o, ok := m.activeOrders[binID]
		m.mu.RUnlock()
		if !ok {
			continue
		}

		// 从数据库重新获取以检查状态是否被外部更新。
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

// --- 辅助函数 ---

// mapBinanceStatus 将 Binance 订单状态字符串映射为本地状态常量。
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

// isActiveStatus 如果订单状态表示仍然活跃则返回 true。
func isActiveStatus(status string) bool {
	return status == OrderStatusNew || status == OrderStatusPartialFilled || status == OrderStatusSubmitted
}

// notifyFailure 发送关于订单失败的通知。
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

// logInfo 安全地以 INFO 级别记录日志。
func (m *OrderManager) logInfo(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Info(msg, fields...)
	}
}

// logWarn 安全地以 WARN 级别记录日志。
func (m *OrderManager) logWarn(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Warn(msg, fields...)
	}
}

// logError 安全地以 ERROR 级别记录日志。
func (m *OrderManager) logError(msg string, fields ...zap.Field) {
	if m.log != nil {
		m.log.Error(msg, fields...)
	}
}
