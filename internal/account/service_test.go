package account

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"

	"money-loves-me/internal/config"
	"money-loves-me/internal/logger"
	"money-loves-me/internal/model"
	"money-loves-me/internal/store"
	"money-loves-me/pkg/binance"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// --- test doubles ---

// mockBinanceClient implements BinanceAccountClient for testing.
type mockBinanceClient struct {
	info *binance.AccountInfo
	err  error
}

func (m *mockBinanceClient) GetAccountInfo() (*binance.AccountInfo, error) {
	return m.info, m.err
}

// mockMarketPricer implements MarketPricer for testing.
type mockMarketPricer struct {
	prices map[string]decimal.Decimal
}

func (m *mockMarketPricer) GetCurrentPrice(symbol string) (decimal.Decimal, error) {
	p, ok := m.prices[symbol]
	if !ok {
		return decimal.Zero, nil
	}
	return p, nil
}

// --- helpers ---

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Trade{}, &model.AccountSnapshot{}, &model.Order{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T, client BinanceAccountClient, pricer MarketPricer) (*AccountService, *store.TradeStore, *store.AccountStore) {
	t.Helper()
	db := setupTestDB(t)
	tradeStore := store.NewTradeStore(db)
	accountStore := store.NewAccountStore(db)

	// Create service without background refresh for deterministic tests.
	svc := &AccountService{
		client:        client,
		market:        pricer,
		tradeStore:    tradeStore,
		accountStore:  accountStore,
		log:           noopLogger(),
		cache:         make(map[string]BalanceInfo),
		cancelRefresh: func() {},
	}
	return svc, tradeStore, accountStore
}

// noopLogger returns a logger that discards output (for tests).
func noopLogger() *logger.Logger {
	// Use a minimal config that writes to nowhere.
	cfg := config.LogConfig{Level: "DEBUG", MaxSizeMB: 100, MaxAgeDays: 30}
	l, err := logger.NewLogger("account-test", cfg)
	if err != nil {
		panic(err)
	}
	return l
}

// --- GetBalances tests ---

func TestGetBalances_ReturnsCachedBalances(t *testing.T) {
	svc, _, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	// Populate cache manually.
	svc.mu.Lock()
	svc.cache["BTC"] = BalanceInfo{Free: decimal.NewFromFloat(1.5), Locked: decimal.NewFromFloat(0.5)}
	svc.cache["ETH"] = BalanceInfo{Free: decimal.NewFromFloat(10.0), Locked: decimal.Zero}
	svc.mu.Unlock()

	balances, err := svc.GetBalances()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(balances))
	}
	if !balances["BTC"].Free.Equal(decimal.NewFromFloat(1.5)) {
		t.Errorf("BTC free = %s, want 1.5", balances["BTC"].Free)
	}
	if !balances["BTC"].Locked.Equal(decimal.NewFromFloat(0.5)) {
		t.Errorf("BTC locked = %s, want 0.5", balances["BTC"].Locked)
	}
}

func TestGetBalances_EmptyCache_ReturnsError(t *testing.T) {
	svc, _, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	_, err := svc.GetBalances()
	if err == nil {
		t.Fatal("expected error for empty cache")
	}
}

// --- GetTotalAssetValue tests ---

func TestGetTotalAssetValue_CalculatesCorrectly(t *testing.T) {
	pricer := &mockMarketPricer{
		prices: map[string]decimal.Decimal{
			"BTCUSDT": decimal.NewFromInt(50000),
			"ETHUSDT": decimal.NewFromInt(3000),
		},
	}
	svc, _, _ := newTestService(t, &mockBinanceClient{}, pricer)

	svc.mu.Lock()
	svc.cache["BTC"] = BalanceInfo{Free: decimal.NewFromInt(2), Locked: decimal.Zero}
	svc.cache["ETH"] = BalanceInfo{Free: decimal.NewFromInt(10), Locked: decimal.Zero}
	svc.cache["USDT"] = BalanceInfo{Free: decimal.NewFromInt(5000), Locked: decimal.Zero}
	svc.mu.Unlock()

	// Expected: 2*50000 + 10*3000 + 5000 = 100000 + 30000 + 5000 = 135000
	total, err := svc.GetTotalAssetValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := decimal.NewFromInt(135000)
	if !total.Equal(expected) {
		t.Errorf("total = %s, want %s", total, expected)
	}
}

// --- GetPositionPnL tests ---

func TestGetPositionPnL_CalculatesWithFees(t *testing.T) {
	svc, tradeStore, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	// Insert test trades.
	now := time.Now()
	trades := []model.Trade{
		{
			OrderID: 1, Symbol: "BTCUSDT", Side: "BUY",
			Price: decimal.NewFromInt(50000), Quantity: decimal.NewFromInt(1),
			Amount: decimal.NewFromInt(50000), Fee: decimal.NewFromInt(50),
			FeeAsset: "USDT", StrategyName: "test", ExecutedAt: now,
		},
		{
			OrderID: 2, Symbol: "BTCUSDT", Side: "SELL",
			Price: decimal.NewFromInt(55000), Quantity: decimal.NewFromInt(1),
			Amount: decimal.NewFromInt(55000), Fee: decimal.NewFromInt(55),
			FeeAsset: "USDT", StrategyName: "test", ExecutedAt: now.Add(time.Hour),
		},
	}
	for i := range trades {
		if err := tradeStore.Create(&trades[i]); err != nil {
			t.Fatalf("failed to create trade: %v", err)
		}
	}

	pnl, err := svc.GetPositionPnL("BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PnL = 55000 - 50000 - (50+55) = 5000 - 105 = 4895
	expectedPnL := decimal.NewFromInt(4895)
	if !pnl.RealizedPnL.Equal(expectedPnL) {
		t.Errorf("realized PnL = %s, want %s", pnl.RealizedPnL, expectedPnL)
	}
	expectedFees := decimal.NewFromInt(105)
	if !pnl.TotalFees.Equal(expectedFees) {
		t.Errorf("total fees = %s, want %s", pnl.TotalFees, expectedFees)
	}
	if pnl.TradeCount != 2 {
		t.Errorf("trade count = %d, want 2", pnl.TradeCount)
	}
}

func TestGetPositionPnL_EmptySymbol_ReturnsError(t *testing.T) {
	svc, _, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	_, err := svc.GetPositionPnL("")
	if err == nil {
		t.Fatal("expected error for empty symbol")
	}
}

// --- GetAssetHistory tests ---

func TestGetAssetHistory_ReturnsSnapshots(t *testing.T) {
	svc, _, accountStore := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	now := time.Now().Truncate(time.Second)
	snap := &model.AccountSnapshot{
		TotalValueUSDT: decimal.NewFromInt(100000),
		Balances:       []byte(`{"BTC":"1.0"}`),
		SnapshotAt:     now,
	}
	if err := accountStore.Create(snap); err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	results, err := svc.GetAssetHistory(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(results))
	}
	if !results[0].TotalValueUSDT.Equal(decimal.NewFromInt(100000)) {
		t.Errorf("total value = %s, want 100000", results[0].TotalValueUSDT)
	}
}

// --- GetFeeStats tests ---

func TestGetFeeStats_CalculatesCumulativeFees(t *testing.T) {
	svc, tradeStore, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

	now := time.Now()
	trades := []model.Trade{
		{
			OrderID: 1, Symbol: "BTCUSDT", Side: "BUY",
			Price: decimal.NewFromInt(50000), Quantity: decimal.NewFromInt(1),
			Amount: decimal.NewFromInt(50000), Fee: decimal.NewFromInt(50),
			FeeAsset: "USDT", StrategyName: "test", ExecutedAt: now,
		},
		{
			OrderID: 2, Symbol: "ETHUSDT", Side: "BUY",
			Price: decimal.NewFromInt(3000), Quantity: decimal.NewFromInt(10),
			Amount: decimal.NewFromInt(30000), Fee: decimal.NewFromInt(30),
			FeeAsset: "USDT", StrategyName: "test", ExecutedAt: now,
		},
		{
			OrderID: 3, Symbol: "BTCUSDT", Side: "SELL",
			Price: decimal.NewFromInt(55000), Quantity: decimal.NewFromInt(1),
			Amount: decimal.NewFromInt(55000), Fee: decimal.NewFromFloat(0.001),
			FeeAsset: "BNB", StrategyName: "test", ExecutedAt: now,
		},
	}
	for i := range trades {
		if err := tradeStore.Create(&trades[i]); err != nil {
			t.Fatalf("failed to create trade: %v", err)
		}
	}

	stats, err := svc.GetFeeStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total = 50 + 30 + 0.001 = 80.001
	expectedTotal := decimal.NewFromFloat(80.001)
	if !stats.TotalFees.Equal(expectedTotal) {
		t.Errorf("total fees = %s, want %s", stats.TotalFees, expectedTotal)
	}
	if stats.TradeCount != 3 {
		t.Errorf("trade count = %d, want 3", stats.TradeCount)
	}
	// Fee by asset: USDT=80, BNB=0.001
	if !stats.FeeByAsset["USDT"].Equal(decimal.NewFromInt(80)) {
		t.Errorf("USDT fees = %s, want 80", stats.FeeByAsset["USDT"])
	}
	if !stats.FeeByAsset["BNB"].Equal(decimal.NewFromFloat(0.001)) {
		t.Errorf("BNB fees = %s, want 0.001", stats.FeeByAsset["BNB"])
	}
	// Fee by symbol: BTCUSDT=50.001, ETHUSDT=30
	expectedBTC := decimal.NewFromFloat(50.001)
	if !stats.FeeBySymbol["BTCUSDT"].Equal(expectedBTC) {
		t.Errorf("BTCUSDT fees = %s, want %s", stats.FeeBySymbol["BTCUSDT"], expectedBTC)
	}
	if !stats.FeeBySymbol["ETHUSDT"].Equal(decimal.NewFromInt(30)) {
		t.Errorf("ETHUSDT fees = %s, want 30", stats.FeeBySymbol["ETHUSDT"])
	}
}

// --- refreshCache tests ---

func TestRefreshCache_UpdatesFromBinanceAPI(t *testing.T) {
	client := &mockBinanceClient{
		info: &binance.AccountInfo{
			Balances: []binance.Balance{
				{Asset: "BTC", Free: "1.5", Locked: "0.5"},
				{Asset: "ETH", Free: "10.0", Locked: "0.0"},
				{Asset: "DOGE", Free: "0.0", Locked: "0.0"}, // zero balance, should be skipped
			},
		},
	}
	svc, _, _ := newTestService(t, client, &mockMarketPricer{})

	svc.refreshCache()

	svc.mu.RLock()
	defer svc.mu.RUnlock()

	if len(svc.cache) != 2 {
		t.Fatalf("expected 2 cached assets, got %d", len(svc.cache))
	}
	if !svc.cache["BTC"].Free.Equal(decimal.NewFromFloat(1.5)) {
		t.Errorf("BTC free = %s, want 1.5", svc.cache["BTC"].Free)
	}
	if _, ok := svc.cache["DOGE"]; ok {
		t.Error("DOGE should not be cached (zero balance)")
	}
}

// Feature: binance-trading-system, Property 10: 总资产价值计算正确性
// **Validates: Requirements 5.3**
func TestProperty10_TotalAssetValueCalculation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of non-USDT assets (0-5).
		numAssets := rapid.IntRange(0, 5).Draw(rt, "numAssets")

		// Pre-defined asset names to avoid duplicates and ensure valid symbols.
		assetPool := []string{"BTC", "ETH", "BNB", "SOL", "ADA", "XRP", "DOT", "AVAX"}

		// Pick unique assets from the pool.
		assets := assetPool[:numAssets]

		// Generate balances and prices for each asset.
		prices := make(map[string]decimal.Decimal)
		cache := make(map[string]BalanceInfo)
		expectedTotal := decimal.Zero

		for _, asset := range assets {
			// Generate free and locked balances as positive integers (avoid floating point issues).
			freeInt := rapid.Int64Range(0, 1000000).Draw(rt, asset+"_free")
			lockedInt := rapid.Int64Range(0, 1000000).Draw(rt, asset+"_locked")
			free := decimal.NewFromInt(freeInt)
			locked := decimal.NewFromInt(lockedInt)

			// Generate a positive price for this asset.
			priceInt := rapid.Int64Range(1, 100000).Draw(rt, asset+"_price")
			price := decimal.NewFromInt(priceInt)

			cache[asset] = BalanceInfo{Free: free, Locked: locked}
			prices[asset+"USDT"] = price

			// Manual expected calculation: (free + locked) * price
			amount := free.Add(locked)
			expectedTotal = expectedTotal.Add(amount.Mul(price))
		}

		// Optionally add a USDT balance (counted at 1:1).
		hasUSDT := rapid.Bool().Draw(rt, "hasUSDT")
		if hasUSDT {
			usdtFreeInt := rapid.Int64Range(0, 1000000).Draw(rt, "USDT_free")
			usdtLockedInt := rapid.Int64Range(0, 1000000).Draw(rt, "USDT_locked")
			usdtFree := decimal.NewFromInt(usdtFreeInt)
			usdtLocked := decimal.NewFromInt(usdtLockedInt)
			cache["USDT"] = BalanceInfo{Free: usdtFree, Locked: usdtLocked}
			expectedTotal = expectedTotal.Add(usdtFree.Add(usdtLocked))
		}

		// Set up the service with the generated data.
		pricer := &mockMarketPricer{prices: prices}
		svc, _, _ := newTestService(t, &mockBinanceClient{}, pricer)

		svc.mu.Lock()
		svc.cache = cache
		svc.mu.Unlock()

		// Call GetTotalAssetValue and verify.
		total, err := svc.GetTotalAssetValue()
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}

		if !total.Equal(expectedTotal) {
			rt.Fatalf("total asset value = %s, want %s", total, expectedTotal)
		}
	})
}

// Feature: binance-trading-system, Property 11: 盈亏计算包含手续费
// **Validates: Requirements 5.4, 5.6, 6.8**
func TestProperty11_PnLCalculationIncludesFees(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, tradeStore, _ := newTestService(t, &mockBinanceClient{}, &mockMarketPricer{})

		symbol := "BTCUSDT"
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		// Generate 1-10 trades with random BUY/SELL sides.
		numTrades := rapid.IntRange(1, 10).Draw(rt, "numTrades")

		var expectedBuyAmount decimal.Decimal
		var expectedSellAmount decimal.Decimal
		var expectedTotalFees decimal.Decimal

		for i := 0; i < numTrades; i++ {
			side := rapid.SampledFrom([]string{"BUY", "SELL"}).Draw(rt, "side")
			// Use integer amounts to avoid floating point precision issues.
			amountInt := rapid.Int64Range(1, 1000000).Draw(rt, "amount")
			feeInt := rapid.Int64Range(0, 10000).Draw(rt, "fee")
			priceInt := rapid.Int64Range(1, 100000).Draw(rt, "price")
			qtyInt := rapid.Int64Range(1, 10000).Draw(rt, "qty")

			amount := decimal.NewFromInt(amountInt)
			fee := decimal.NewFromInt(feeInt)
			price := decimal.NewFromInt(priceInt)
			qty := decimal.NewFromInt(qtyInt)

			trade := &model.Trade{
				OrderID:      int64(i + 1),
				Symbol:       symbol,
				Side:         side,
				Price:        price,
				Quantity:     qty,
				Amount:       amount,
				Fee:          fee,
				FeeAsset:     "USDT",
				StrategyName: "test-strategy",
				ExecutedAt:   baseTime.Add(time.Duration(i) * time.Minute),
			}
			if err := tradeStore.Create(trade); err != nil {
				rt.Fatalf("failed to create trade: %v", err)
			}

			switch side {
			case "BUY":
				expectedBuyAmount = expectedBuyAmount.Add(amount)
			case "SELL":
				expectedSellAmount = expectedSellAmount.Add(amount)
			}
			expectedTotalFees = expectedTotalFees.Add(fee)
		}

		// Verify GetPositionPnL: RealizedPnL == SellAmount - BuyAmount - TotalFees
		pnl, err := svc.GetPositionPnL(symbol)
		if err != nil {
			rt.Fatalf("GetPositionPnL error: %v", err)
		}

		expectedPnL := expectedSellAmount.Sub(expectedBuyAmount).Sub(expectedTotalFees)

		if !pnl.RealizedPnL.Equal(expectedPnL) {
			rt.Fatalf("RealizedPnL = %s, want %s (sell=%s, buy=%s, fees=%s)",
				pnl.RealizedPnL, expectedPnL, expectedSellAmount, expectedBuyAmount, expectedTotalFees)
		}
		if !pnl.BuyAmount.Equal(expectedBuyAmount) {
			rt.Fatalf("BuyAmount = %s, want %s", pnl.BuyAmount, expectedBuyAmount)
		}
		if !pnl.SellAmount.Equal(expectedSellAmount) {
			rt.Fatalf("SellAmount = %s, want %s", pnl.SellAmount, expectedSellAmount)
		}
		if !pnl.TotalFees.Equal(expectedTotalFees) {
			rt.Fatalf("PnL TotalFees = %s, want %s", pnl.TotalFees, expectedTotalFees)
		}
		if pnl.TradeCount != numTrades {
			rt.Fatalf("TradeCount = %d, want %d", pnl.TradeCount, numTrades)
		}

		// Verify GetFeeStats: TotalFees == sum of all individual trade fees
		stats, err := svc.GetFeeStats()
		if err != nil {
			rt.Fatalf("GetFeeStats error: %v", err)
		}

		if !stats.TotalFees.Equal(expectedTotalFees) {
			rt.Fatalf("FeeStats TotalFees = %s, want %s", stats.TotalFees, expectedTotalFees)
		}
		if stats.TradeCount != numTrades {
			rt.Fatalf("FeeStats TradeCount = %d, want %d", stats.TradeCount, numTrades)
		}
	})
}
