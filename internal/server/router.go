package server

import (
	"github.com/gin-gonic/gin"
)

// Server holds all dependencies for the HTTP server.
type Server struct {
	router  *gin.Engine
	auth    *AuthService
	handler *Handler
}

// NewServer creates a new HTTP server with all routes configured.
func NewServer(auth *AuthService, handler *Handler) *Server {
	s := &Server{
		router:  gin.Default(),
		auth:    auth,
		handler: handler,
	}
	s.setupRoutes()
	return s
}

// Router returns the underlying gin.Engine for testing.
func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")

	// Public routes
	api.POST("/auth/login", s.handler.Login)

	// Protected routes
	protected := api.Group("")
	protected.Use(s.auth.JWTAuthMiddleware())
	{
		// Market
		protected.GET("/market/klines/:symbol", s.handler.GetKlines)
		protected.GET("/market/orderbook/:symbol", s.handler.GetOrderBook)

		// Orders
		protected.POST("/orders", s.handler.CreateOrder)
		protected.DELETE("/orders/:id", s.handler.CancelOrder)
		protected.GET("/orders", s.handler.GetOrders)
		protected.GET("/orders/export", s.handler.ExportOrders)

		// Account
		protected.GET("/account/balances", s.handler.GetBalances)
		protected.GET("/account/pnl", s.handler.GetPnL)
		protected.GET("/account/fees", s.handler.GetFeeStats)

		// Strategy
		protected.POST("/strategy/start", s.handler.StartStrategy)
		protected.POST("/strategy/stop", s.handler.StopStrategy)
		protected.GET("/strategy/status", s.handler.GetStrategyStatus)

		// Risk
		protected.GET("/risk/config", s.handler.GetRiskConfig)
		protected.PUT("/risk/config", s.handler.UpdateRiskConfig)

		// Backtest
		protected.POST("/backtest/run", s.handler.RunBacktest)
		protected.GET("/backtest/results", s.handler.GetBacktestResults)

		// Optimizer
		protected.GET("/optimizer/history", s.handler.GetOptimizerHistory)

		// Notifications
		protected.GET("/notifications", s.handler.GetNotifications)
		protected.PUT("/notifications/:id/read", s.handler.MarkNotificationRead)
		protected.PUT("/notifications/settings", s.handler.UpdateNotificationSettings)

		// Trades
		protected.GET("/trades", s.handler.GetTrades)
	}
}

// Run starts the HTTP server on the given address.
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
