package store

import (
	"encoding/json"
	"testing"
	"time"

	"money-loves-me/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

// setupTestDB creates an in-memory SQLite database with all tables migrated.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db
}

// genDecimal generates a random decimal with up to 8 decimal places in a reasonable range.
func genDecimal(t *rapid.T, label string) decimal.Decimal {
	intPart := rapid.Int64Range(0, 999999).Draw(t, label+"_int")
	fracPart := rapid.Int64Range(0, 99999999).Draw(t, label+"_frac")
	s := decimal.NewFromInt(intPart).Add(decimal.NewFromInt(fracPart).Div(decimal.NewFromInt(100000000)))
	return s
}

// genSide generates a random order side.
func genSide(t *rapid.T) string {
	return rapid.SampledFrom([]string{"BUY", "SELL"}).Draw(t, "side")
}

// genOrderType generates a random order type.
func genOrderType(t *rapid.T) string {
	return rapid.SampledFrom([]string{"LIMIT", "MARKET", "STOP_LOSS_LIMIT", "TAKE_PROFIT_LIMIT"}).Draw(t, "order_type")
}

// genOrderStatus generates a random order status.
func genOrderStatus(t *rapid.T) string {
	return rapid.SampledFrom([]string{"SUBMITTED", "PARTIAL", "FILLED", "CANCELLED"}).Draw(t, "status")
}

// genSymbol generates a random trading pair symbol.
func genSymbol(t *rapid.T) string {
	return rapid.SampledFrom([]string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "ADAUSDT"}).Draw(t, "symbol")
}

// genStrategyName generates a random strategy name.
func genStrategyName(t *rapid.T) string {
	return rapid.SampledFrom([]string{"ma_cross", "rsi", "bollinger", "grid", "dca"}).Draw(t, "strategy_name")
}

// genFeeAsset generates a random fee asset.
func genFeeAsset(t *rapid.T) string {
	return rapid.SampledFrom([]string{"BNB", "USDT", "BTC", "ETH"}).Draw(t, "fee_asset")
}

// genDecisionReason generates a random JSON decision reason.
func genDecisionReason(t *rapid.T) json.RawMessage {
	reason := model.DecisionReasonJSON{
		Indicators: map[string]float64{
			"MA7":  rapid.Float64Range(100, 50000).Draw(t, "ma7"),
			"MA25": rapid.Float64Range(100, 50000).Draw(t, "ma25"),
			"RSI":  rapid.Float64Range(0, 100).Draw(t, "rsi"),
		},
		TriggerRule: rapid.SampledFrom([]string{
			"MA7 crosses above MA25",
			"RSI below 30",
			"Price breaks upper Bollinger band",
		}).Draw(t, "trigger_rule"),
		MarketState: rapid.SampledFrom([]string{
			"uptrend",
			"downtrend",
			"sideways",
		}).Draw(t, "market_state"),
	}
	data, _ := json.Marshal(reason)
	return data
}

// genJSONParams generates random JSON params for backtest/optimization.
func genJSONParams(t *rapid.T, label string) json.RawMessage {
	params := map[string]interface{}{
		"period": rapid.IntRange(1, 100).Draw(t, label+"_period"),
		"mult":   rapid.Float64Range(0.5, 5.0).Draw(t, label+"_mult"),
	}
	data, _ := json.Marshal(params)
	return data
}

// genJSONMetrics generates random JSON metrics for optimization records.
func genJSONMetrics(t *rapid.T, label string) json.RawMessage {
	metrics := map[string]interface{}{
		"total_return": rapid.Float64Range(-50, 200).Draw(t, label+"_return"),
		"win_rate":     rapid.Float64Range(0, 100).Draw(t, label+"_winrate"),
	}
	data, _ := json.Marshal(metrics)
	return data
}

// assertDecimalEqual checks that two decimals are equal using shopspring's Equal method.
func assertDecimalEqual(t *testing.T, expected, actual decimal.Decimal, field string) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("%s mismatch: expected %s, got %s", field, expected.String(), actual.String())
	}
}

// assertJSONEqual checks that two JSON raw messages are semantically equal.
func assertJSONEqual(t *testing.T, expected, actual json.RawMessage, field string) {
	t.Helper()
	if expected == nil && actual == nil {
		return
	}
	var e, a interface{}
	if err := json.Unmarshal(expected, &e); err != nil {
		t.Errorf("%s: failed to unmarshal expected: %v", field, err)
		return
	}
	if err := json.Unmarshal(actual, &a); err != nil {
		t.Errorf("%s: failed to unmarshal actual: %v", field, err)
		return
	}
	eb, _ := json.Marshal(e)
	ab, _ := json.Marshal(a)
	if string(eb) != string(ab) {
		t.Errorf("%s mismatch:\n  expected: %s\n  actual:   %s", field, string(eb), string(ab))
	}
}

// Feature: binance-trading-system, Property 5: 数据持久化往返
// Validates: Requirements 9.1, 3.6, 10.6
//
// Property 5: For any trade record, order record, backtest result, or optimization record,
// writing to the database and reading back should yield an equivalent record.

func TestProperty5_OrderPersistenceRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	orderStore := NewOrderStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		order := &model.Order{
			Symbol:       genSymbol(rt),
			Side:         genSide(rt),
			Type:         genOrderType(rt),
			Quantity:     genDecimal(rt, "quantity"),
			Price:        genDecimal(rt, "price"),
			StopPrice:    genDecimal(rt, "stop_price"),
			Status:       genOrderStatus(rt),
			Fee:          genDecimal(rt, "fee"),
			FeeAsset:     genFeeAsset(rt),
			StrategyName: genStrategyName(rt),
		}

		// Write
		if err := orderStore.Create(order); err != nil {
			t.Fatalf("failed to create order: %v", err)
		}
		if order.ID == 0 {
			t.Fatal("order ID should be set after creation")
		}

		// Read back
		got, err := orderStore.GetByID(order.ID)
		if err != nil {
			t.Fatalf("failed to read order: %v", err)
		}

		// Compare all fields
		if got.Symbol != order.Symbol {
			t.Errorf("Symbol mismatch: expected %s, got %s", order.Symbol, got.Symbol)
		}
		if got.Side != order.Side {
			t.Errorf("Side mismatch: expected %s, got %s", order.Side, got.Side)
		}
		if got.Type != order.Type {
			t.Errorf("Type mismatch: expected %s, got %s", order.Type, got.Type)
		}
		assertDecimalEqual(t, order.Quantity, got.Quantity, "Quantity")
		assertDecimalEqual(t, order.Price, got.Price, "Price")
		assertDecimalEqual(t, order.StopPrice, got.StopPrice, "StopPrice")
		if got.Status != order.Status {
			t.Errorf("Status mismatch: expected %s, got %s", order.Status, got.Status)
		}
		assertDecimalEqual(t, order.Fee, got.Fee, "Fee")
		if got.FeeAsset != order.FeeAsset {
			t.Errorf("FeeAsset mismatch: expected %s, got %s", order.FeeAsset, got.FeeAsset)
		}
		if got.StrategyName != order.StrategyName {
			t.Errorf("StrategyName mismatch: expected %s, got %s", order.StrategyName, got.StrategyName)
		}
	})
}

func TestProperty5_TradePersistenceRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	orderStore := NewOrderStore(db)
	tradeStore := NewTradeStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		// Create a parent order first (FK constraint)
		order := &model.Order{
			Symbol:       genSymbol(rt),
			Side:         genSide(rt),
			Type:         "MARKET",
			Quantity:     genDecimal(rt, "order_qty"),
			Price:        genDecimal(rt, "order_price"),
			Status:       "FILLED",
			StrategyName: genStrategyName(rt),
		}
		if err := orderStore.Create(order); err != nil {
			t.Fatalf("failed to create parent order: %v", err)
		}

		executedAt := time.Now().UTC().Truncate(time.Second)

		trade := &model.Trade{
			OrderID:        order.ID,
			Symbol:         order.Symbol,
			Side:           order.Side,
			Price:          genDecimal(rt, "trade_price"),
			Quantity:       genDecimal(rt, "trade_qty"),
			Amount:         genDecimal(rt, "trade_amount"),
			Fee:            genDecimal(rt, "trade_fee"),
			FeeAsset:       genFeeAsset(rt),
			StrategyName:   order.StrategyName,
			DecisionReason: genDecisionReason(rt),
			BalanceBefore:  genDecimal(rt, "balance_before"),
			BalanceAfter:   genDecimal(rt, "balance_after"),
			ExecutedAt:     executedAt,
		}

		// Write
		if err := tradeStore.Create(trade); err != nil {
			t.Fatalf("failed to create trade: %v", err)
		}
		if trade.ID == 0 {
			t.Fatal("trade ID should be set after creation")
		}

		// Read back
		trades, err := tradeStore.GetByOrderID(order.ID)
		if err != nil {
			t.Fatalf("failed to read trades: %v", err)
		}
		if len(trades) == 0 {
			t.Fatal("expected at least one trade")
		}

		got := trades[0]

		// Compare all fields
		if got.OrderID != trade.OrderID {
			t.Errorf("OrderID mismatch: expected %d, got %d", trade.OrderID, got.OrderID)
		}
		if got.Symbol != trade.Symbol {
			t.Errorf("Symbol mismatch: expected %s, got %s", trade.Symbol, got.Symbol)
		}
		if got.Side != trade.Side {
			t.Errorf("Side mismatch: expected %s, got %s", trade.Side, got.Side)
		}
		assertDecimalEqual(t, trade.Price, got.Price, "Price")
		assertDecimalEqual(t, trade.Quantity, got.Quantity, "Quantity")
		assertDecimalEqual(t, trade.Amount, got.Amount, "Amount")
		assertDecimalEqual(t, trade.Fee, got.Fee, "Fee")
		if got.FeeAsset != trade.FeeAsset {
			t.Errorf("FeeAsset mismatch: expected %s, got %s", trade.FeeAsset, got.FeeAsset)
		}
		if got.StrategyName != trade.StrategyName {
			t.Errorf("StrategyName mismatch: expected %s, got %s", trade.StrategyName, got.StrategyName)
		}
		assertJSONEqual(t, trade.DecisionReason, got.DecisionReason, "DecisionReason")
		assertDecimalEqual(t, trade.BalanceBefore, got.BalanceBefore, "BalanceBefore")
		assertDecimalEqual(t, trade.BalanceAfter, got.BalanceAfter, "BalanceAfter")
		if !got.ExecutedAt.Equal(trade.ExecutedAt) {
			t.Errorf("ExecutedAt mismatch: expected %v, got %v", trade.ExecutedAt, got.ExecutedAt)
		}
	})
}
func TestProperty5_BacktestResultPersistenceRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	backtestStore := NewBacktestStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		startTime := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
		endTime := time.Now().UTC().Truncate(time.Second)

		result := &model.BacktestResult{
			StrategyName:   genStrategyName(rt),
			Symbol:         genSymbol(rt),
			Params:         genJSONParams(rt, "bt_params"),
			StartTime:      startTime,
			EndTime:        endTime,
			InitialCapital: genDecimal(rt, "initial_cap"),
			TotalReturn:    genDecimal(rt, "total_return"),
			NetProfit:      genDecimal(rt, "net_profit"),
			MaxDrawdown:    genDecimal(rt, "max_drawdown"),
			WinRate:        genDecimal(rt, "win_rate"),
			ProfitFactor:   genDecimal(rt, "profit_factor"),
			TotalTrades:    rapid.IntRange(0, 1000).Draw(rt, "total_trades"),
			TotalFees:      genDecimal(rt, "total_fees"),
			EquityCurve:    json.RawMessage(`[{"time":"2024-01-01","value":10000}]`),
			Trades:         json.RawMessage(`[{"price":"100","qty":"1"}]`),
			Slippage:       genDecimal(rt, "slippage"),
		}

		// Write
		if err := backtestStore.Create(result); err != nil {
			t.Fatalf("failed to create backtest result: %v", err)
		}
		if result.ID == 0 {
			t.Fatal("backtest result ID should be set after creation")
		}

		// Read back
		got, err := backtestStore.GetByID(result.ID)
		if err != nil {
			t.Fatalf("failed to read backtest result: %v", err)
		}

		// Compare all fields
		if got.StrategyName != result.StrategyName {
			t.Errorf("StrategyName mismatch: expected %s, got %s", result.StrategyName, got.StrategyName)
		}
		if got.Symbol != result.Symbol {
			t.Errorf("Symbol mismatch: expected %s, got %s", result.Symbol, got.Symbol)
		}
		assertJSONEqual(t, result.Params, got.Params, "Params")
		if !got.StartTime.Equal(result.StartTime) {
			t.Errorf("StartTime mismatch: expected %v, got %v", result.StartTime, got.StartTime)
		}
		if !got.EndTime.Equal(result.EndTime) {
			t.Errorf("EndTime mismatch: expected %v, got %v", result.EndTime, got.EndTime)
		}
		assertDecimalEqual(t, result.InitialCapital, got.InitialCapital, "InitialCapital")
		assertDecimalEqual(t, result.TotalReturn, got.TotalReturn, "TotalReturn")
		assertDecimalEqual(t, result.NetProfit, got.NetProfit, "NetProfit")
		assertDecimalEqual(t, result.MaxDrawdown, got.MaxDrawdown, "MaxDrawdown")
		assertDecimalEqual(t, result.WinRate, got.WinRate, "WinRate")
		assertDecimalEqual(t, result.ProfitFactor, got.ProfitFactor, "ProfitFactor")
		if got.TotalTrades != result.TotalTrades {
			t.Errorf("TotalTrades mismatch: expected %d, got %d", result.TotalTrades, got.TotalTrades)
		}
		assertDecimalEqual(t, result.TotalFees, got.TotalFees, "TotalFees")
		assertJSONEqual(t, result.EquityCurve, got.EquityCurve, "EquityCurve")
		assertJSONEqual(t, result.Trades, got.Trades, "Trades")
		assertDecimalEqual(t, result.Slippage, got.Slippage, "Slippage")
	})
}

func TestProperty5_OptimizationRecordPersistenceRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	optStore := NewOptimizationStore(db)

	rapid.Check(t, func(rt *rapid.T) {
		notes := rapid.SampledFrom([]string{
			"Strategy performed well in uptrend",
			"Parameters adjusted for volatility",
			"No significant improvement found",
		}).Draw(rt, "notes")

		record := &model.OptimizationRecord{
			StrategyName:  genStrategyName(rt),
			OldParams:     genJSONParams(rt, "old_params"),
			NewParams:     genJSONParams(rt, "new_params"),
			OldMetrics:    genJSONMetrics(rt, "old_metrics"),
			NewMetrics:    genJSONMetrics(rt, "new_metrics"),
			AnalysisNotes: &notes,
			Applied:       rapid.Bool().Draw(rt, "applied"),
		}

		// Write
		if err := optStore.Create(record); err != nil {
			t.Fatalf("failed to create optimization record: %v", err)
		}
		if record.ID == 0 {
			t.Fatal("optimization record ID should be set after creation")
		}

		// Read back
		records, err := optStore.GetByStrategy(record.StrategyName)
		if err != nil {
			t.Fatalf("failed to read optimization records: %v", err)
		}

		// Find our record by ID
		var got *model.OptimizationRecord
		for i := range records {
			if records[i].ID == record.ID {
				got = &records[i]
				break
			}
		}
		if got == nil {
			t.Fatal("optimization record not found after creation")
		}

		// Compare all fields
		if got.StrategyName != record.StrategyName {
			t.Errorf("StrategyName mismatch: expected %s, got %s", record.StrategyName, got.StrategyName)
		}
		assertJSONEqual(t, record.OldParams, got.OldParams, "OldParams")
		assertJSONEqual(t, record.NewParams, got.NewParams, "NewParams")
		assertJSONEqual(t, record.OldMetrics, got.OldMetrics, "OldMetrics")
		assertJSONEqual(t, record.NewMetrics, got.NewMetrics, "NewMetrics")
		if got.AnalysisNotes == nil || *got.AnalysisNotes != *record.AnalysisNotes {
			t.Errorf("AnalysisNotes mismatch: expected %v, got %v", record.AnalysisNotes, got.AnalysisNotes)
		}
		if got.Applied != record.Applied {
			t.Errorf("Applied mismatch: expected %v, got %v", record.Applied, got.Applied)
		}
	})
}
