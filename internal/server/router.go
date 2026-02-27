package server

import (
	"github.com/gin-gonic/gin"
)

// Server 持有 HTTP 服务器的所有依赖。
type Server struct {
	router  *gin.Engine
	auth    *AuthService
	handler *Handler
}

// NewServer 创建一个配置了所有路由的新 HTTP 服务器。
func NewServer(auth *AuthService, handler *Handler) *Server {
	s := &Server{
		router:  gin.Default(),
		auth:    auth,
		handler: handler,
	}
	s.setupRoutes()
	return s
}

// Router 返回底层的 gin.Engine，用于测试。
func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")

	// 公开路由
	api.POST("/auth/login", s.handler.Login)

	// 受保护路由
	protected := api.Group("")
	protected.Use(s.auth.JWTAuthMiddleware())
	{
		// 行情
		protected.GET("/market/klines/:symbol", s.handler.GetKlines)
		protected.GET("/market/orderbook/:symbol", s.handler.GetOrderBook)

		// 订单
		protected.POST("/orders", s.handler.CreateOrder)
		protected.DELETE("/orders/:id", s.handler.CancelOrder)
		protected.GET("/orders", s.handler.GetOrders)
		protected.GET("/orders/export", s.handler.ExportOrders)

		// 账户
		protected.GET("/account/balances", s.handler.GetBalances)
		protected.GET("/account/pnl", s.handler.GetPnL)
		protected.GET("/account/fees", s.handler.GetFeeStats)

		// 策略
		protected.POST("/strategy/start", s.handler.StartStrategy)
		protected.POST("/strategy/stop", s.handler.StopStrategy)
		protected.GET("/strategy/status", s.handler.GetStrategyStatus)

		// 风控
		protected.GET("/risk/config", s.handler.GetRiskConfig)
		protected.PUT("/risk/config", s.handler.UpdateRiskConfig)

		// 回测
		protected.POST("/backtest/run", s.handler.RunBacktest)
		protected.GET("/backtest/results", s.handler.GetBacktestResults)

		// 优化器
		protected.GET("/optimizer/history", s.handler.GetOptimizerHistory)

		// 通知
		protected.GET("/notifications", s.handler.GetNotifications)
		protected.PUT("/notifications/:id/read", s.handler.MarkNotificationRead)
		protected.PUT("/notifications/settings", s.handler.UpdateNotificationSettings)

		// 交易记录
		protected.GET("/trades", s.handler.GetTrades)
	}
}

// Run 在指定地址启动 HTTP 服务器。
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
