package order

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"money-loves-me/internal/model"
)

// Feature: binance-trading-system, Property 28: CSV 导出往返
// For any set of trade records, exporting to CSV and then parsing back
// should yield equivalent data.
//
// **Validates: Requirements 9.5**

func genTrade(rt *rapid.T, idx int) model.Trade {
	symbol := rapid.SampledFrom([]string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}).Draw(rt, "symbol")
	side := rapid.SampledFrom([]string{"BUY", "SELL"}).Draw(rt, "side")
	strategyName := rapid.SampledFrom([]string{"MA_CROSS", "RSI", "BOLLINGER"}).Draw(rt, "strategy")

	price := decimal.NewFromInt(rapid.Int64Range(1, 999999).Draw(rt, "price")).
		Div(decimal.NewFromInt(100))
	quantity := decimal.NewFromInt(rapid.Int64Range(1, 99999).Draw(rt, "qty")).
		Div(decimal.NewFromInt(10000))
	amount := price.Mul(quantity)
	fee := decimal.NewFromInt(rapid.Int64Range(1, 1000).Draw(rt, "fee")).
		Div(decimal.NewFromInt(10000))
	feeAsset := rapid.SampledFrom([]string{"USDT", "BNB", "BTC"}).Draw(rt, "feeAsset")

	balBefore := decimal.NewFromInt(rapid.Int64Range(100, 999999).Draw(rt, "balBefore")).
		Div(decimal.NewFromInt(100))
	balAfter := decimal.NewFromInt(rapid.Int64Range(100, 999999).Draw(rt, "balAfter")).
		Div(decimal.NewFromInt(100))

	reason := model.DecisionReasonJSON{
		Indicators:  map[string]float64{"MA7": 42350.5, "RSI": 65.3},
		TriggerRule: "MA7 crossed above MA25",
		MarketState: "Uptrend",
	}
	reasonJSON, _ := json.Marshal(reason)

	// Use a fixed base time and offset by index to ensure unique, deterministic times.
	baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	executedAt := baseTime.Add(time.Duration(idx) * time.Minute)

	return model.Trade{
		ID:             int64(idx + 1),
		OrderID:        rapid.Int64Range(1, 100000).Draw(rt, "orderID"),
		Symbol:         symbol,
		Side:           side,
		Price:          price,
		Quantity:       quantity,
		Amount:         amount,
		Fee:            fee,
		FeeAsset:       feeAsset,
		StrategyName:   strategyName,
		DecisionReason: reasonJSON,
		BalanceBefore:  balBefore,
		BalanceAfter:   balAfter,
		ExecutedAt:     executedAt,
	}
}

func TestProperty28_CSVExportRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of trades (0 to 20).
		count := rapid.IntRange(0, 20).Draw(rt, "tradeCount")
		trades := make([]model.Trade, count)
		for i := 0; i < count; i++ {
			trades[i] = genTrade(rt, i)
		}

		// Export to CSV.
		var buf bytes.Buffer
		err := WriteTradesCSV(trades, &buf)
		require.NoError(rt, err, "WriteTradesCSV should not fail")

		// Parse back from CSV.
		parsed, err := ParseTradesCSV(&buf)
		require.NoError(rt, err, "ParseTradesCSV should not fail")

		// Verify equivalence.
		require.Equal(rt, len(trades), len(parsed),
			"parsed trade count should match original")

		for i := range trades {
			orig := trades[i]
			got := parsed[i]

			assert.Equal(rt, orig.ID, got.ID, "ID mismatch at index %d", i)
			assert.Equal(rt, orig.OrderID, got.OrderID, "OrderID mismatch at index %d", i)
			assert.Equal(rt, orig.Symbol, got.Symbol, "Symbol mismatch at index %d", i)
			assert.Equal(rt, orig.Side, got.Side, "Side mismatch at index %d", i)
			assert.True(rt, orig.Price.Equal(got.Price),
				"Price mismatch at index %d: %s vs %s", i, orig.Price, got.Price)
			assert.True(rt, orig.Quantity.Equal(got.Quantity),
				"Quantity mismatch at index %d: %s vs %s", i, orig.Quantity, got.Quantity)
			assert.True(rt, orig.Amount.Equal(got.Amount),
				"Amount mismatch at index %d: %s vs %s", i, orig.Amount, got.Amount)
			assert.True(rt, orig.Fee.Equal(got.Fee),
				"Fee mismatch at index %d: %s vs %s", i, orig.Fee, got.Fee)
			assert.Equal(rt, orig.FeeAsset, got.FeeAsset, "FeeAsset mismatch at index %d", i)
			assert.Equal(rt, orig.StrategyName, got.StrategyName, "StrategyName mismatch at index %d", i)
			assert.True(rt, orig.BalanceBefore.Equal(got.BalanceBefore),
				"BalanceBefore mismatch at index %d: %s vs %s", i, orig.BalanceBefore, got.BalanceBefore)
			assert.True(rt, orig.BalanceAfter.Equal(got.BalanceAfter),
				"BalanceAfter mismatch at index %d: %s vs %s", i, orig.BalanceAfter, got.BalanceAfter)
			assert.True(rt, orig.ExecutedAt.Equal(got.ExecutedAt),
				"ExecutedAt mismatch at index %d: %s vs %s", i, orig.ExecutedAt, got.ExecutedAt)

			// Verify DecisionReason JSON equivalence.
			var origReason, gotReason model.DecisionReasonJSON
			require.NoError(rt, json.Unmarshal(orig.DecisionReason, &origReason))
			require.NoError(rt, json.Unmarshal(got.DecisionReason, &gotReason))
			assert.Equal(rt, origReason, gotReason, "DecisionReason mismatch at index %d", i)
		}
	})
}

func TestProperty28_EmptyTradeSet(t *testing.T) {
	// Edge case: exporting zero trades should produce only a header,
	// and parsing back should yield an empty slice.
	var buf bytes.Buffer
	err := WriteTradesCSV(nil, &buf)
	require.NoError(t, err)

	parsed, err := ParseTradesCSV(&buf)
	require.NoError(t, err)
	assert.Empty(t, parsed)
}
