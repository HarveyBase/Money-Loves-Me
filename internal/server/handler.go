package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 持有所有 HTTP 处理方法。
type Handler struct {
	auth *AuthService
}

// NewHandler 创建一个新的 Handler。
func NewHandler(auth *AuthService) *Handler {
	return &Handler{auth: auth}
}

// Login 处理 POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	resp, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetKlines 处理 GET /api/v1/market/klines/:symbol
func (h *Handler) GetKlines(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"symbol": c.Param("symbol"), "klines": []interface{}{}})
}

// GetOrderBook 处理 GET /api/v1/market/orderbook/:symbol
func (h *Handler) GetOrderBook(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"symbol": c.Param("symbol"), "bids": []interface{}{}, "asks": []interface{}{}})
}

// CreateOrder 处理 POST /api/v1/orders
func (h *Handler) CreateOrder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "order created"})
}

// CancelOrder 处理 DELETE /api/v1/orders/:id
func (h *Handler) CancelOrder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "order cancelled", "id": c.Param("id")})
}

// GetOrders 处理 GET /api/v1/orders
func (h *Handler) GetOrders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"orders": []interface{}{}})
}

// ExportOrders 处理 GET /api/v1/orders/export
func (h *Handler) ExportOrders(c *gin.Context) {
	c.Header("Content-Type", "text/csv")
	c.String(http.StatusOK, "")
}

// GetBalances 处理 GET /api/v1/account/balances
func (h *Handler) GetBalances(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"balances": []interface{}{}})
}

// GetPnL 处理 GET /api/v1/account/pnl
func (h *Handler) GetPnL(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"pnl": "0"})
}

// GetFeeStats 处理 GET /api/v1/account/fees
func (h *Handler) GetFeeStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"total_fees": "0"})
}

// StartStrategy 处理 POST /api/v1/strategy/start
func (h *Handler) StartStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "strategy started"})
}

// StopStrategy 处理 POST /api/v1/strategy/stop
func (h *Handler) StopStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "strategy stopped"})
}

// GetStrategyStatus 处理 GET /api/v1/strategy/status
func (h *Handler) GetStrategyStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"running": false, "strategies": []interface{}{}})
}

// GetRiskConfig 处理 GET /api/v1/risk/config
func (h *Handler) GetRiskConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"config": gin.H{}})
}

// UpdateRiskConfig 处理 PUT /api/v1/risk/config
func (h *Handler) UpdateRiskConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "risk config updated"})
}

// RunBacktest 处理 POST /api/v1/backtest/run
func (h *Handler) RunBacktest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "backtest started"})
}

// GetBacktestResults 处理 GET /api/v1/backtest/results
func (h *Handler) GetBacktestResults(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"results": []interface{}{}})
}

// GetOptimizerHistory 处理 GET /api/v1/optimizer/history
func (h *Handler) GetOptimizerHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"history": []interface{}{}})
}

// GetNotifications 处理 GET /api/v1/notifications
func (h *Handler) GetNotifications(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"notifications": []interface{}{}})
}

// MarkNotificationRead 处理 PUT /api/v1/notifications/:id/read
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "marked as read", "id": c.Param("id")})
}

// UpdateNotificationSettings 处理 PUT /api/v1/notifications/settings
func (h *Handler) UpdateNotificationSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

// GetTrades 处理 GET /api/v1/trades
func (h *Handler) GetTrades(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"trades": []interface{}{}})
}
