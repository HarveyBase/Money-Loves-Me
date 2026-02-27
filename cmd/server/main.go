package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"money-loves-me/internal/config"
	applogger "money-loves-me/internal/logger"
	"money-loves-me/internal/model"
	"money-loves-me/internal/server"
	"money-loves-me/internal/store"
)

func main() {
	fmt.Println("Money Loves Me - Binance Trading System")

	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := applogger.NewLogger("system", cfg.Log)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()
	logger.Info("Starting trading system...")

	// Initialize database
	db, err := model.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	logger.Info("Database connected and migrated")

	// Initialize stores
	userStore := store.NewUserStore(db)
	_ = store.NewStrategyStore(db)
	_ = store.NewOrderStore(db)
	_ = store.NewTradeStore(db)
	_ = store.NewBacktestStore(db)
	_ = store.NewOptimizationStore(db)
	_ = store.NewNotificationStore(db)
	_ = store.NewRiskStore(db)
	_ = store.NewAccountStore(db)

	// Initialize auth and HTTP handler
	authService := server.NewAuthService(userStore)
	handler := server.NewHandler(authService)

	// Initialize WebSocket hub
	wsHub := server.NewWebSocketHub()
	go wsHub.Run()

	// Create HTTP server
	srv := server.NewServer(authService, handler)
	srv.Router().GET("/ws", server.HandleWebSocket(wsHub))

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Router(),
	}

	go func() {
		logger.Info(fmt.Sprintf("HTTP server starting on %s", addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error(fmt.Sprintf("Server shutdown error: %v", err))
	}

	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	logger.Info("System stopped")
}
