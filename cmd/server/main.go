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

	// 支持通过命令行参数指定配置文件，默认使用 configs/config.yaml
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger, err := applogger.NewLogger("system", cfg.Log)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()
	logger.Info("Starting trading system...")

	// 初始化数据库
	db, err := model.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	logger.Info("Database connected and migrated")

	// 初始化存储层
	userStore := store.NewUserStore(db)
	_ = store.NewStrategyStore(db)
	_ = store.NewOrderStore(db)
	_ = store.NewTradeStore(db)
	_ = store.NewBacktestStore(db)
	_ = store.NewOptimizationStore(db)
	_ = store.NewNotificationStore(db)
	_ = store.NewRiskStore(db)
	_ = store.NewAccountStore(db)

	// 初始化认证服务和 HTTP 处理器
	authService := server.NewAuthService(userStore)
	handler := server.NewHandler(authService)

	// 初始化 WebSocket Hub
	wsHub := server.NewWebSocketHub()
	go wsHub.Run()

	// 创建 HTTP 服务器
	srv := server.NewServer(authService, handler)
	srv.Router().GET("/ws", server.HandleWebSocket(wsHub))

	// 启动 HTTP 服务器
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

	// 优雅关闭
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
