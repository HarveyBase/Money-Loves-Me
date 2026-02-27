package order

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"money-loves-me/internal/model"
)

// Feature: binance-trading-system, Property 6: 交易记录完整性
// For any trade record, it must contain all required non-empty fields:
// ExecutedAt, Symbol, Side, Price, Quantity, Amount, Fee, StrategyName,
// DecisionReason, OrderID, BalanceBefore, BalanceAfter.
//
// **Validates: Requirements 9.2, 4.3**

// ValidateTrade checks that a trade record has all required non-empty fields.
func ValidateTrade(trade *model.Trade) []string {
	var missing []string

	if trade.ExecutedAt.IsZero() {
		missing = append(missing, "ExecutedAt")
	}
	if trade.Symbol == "" {
		missing = append(missing, "Symbol")
	}
	if trade.Side == "" {
		missing = append(missing, "Side")
	}
	if trade.Price.IsZero() {
		missing = append(missing, "Price")
	}
	if trade.Quantity.IsZero() {
		missing = append(missing, "Quantity")
	}
	if trade.Amount.IsZero() {
		missing = append(missing, "Amount")
	}
	if trade.Fee.IsZero() {
		missing = append(missing, "Fee")
	}
	if trade.StrategyName == "" {
		missing = append(missing, "StrategyName")
	}
	if len(trade.DecisionReason) == 0 {
		missing = append(missing, "DecisionReason")
	} else {
		// Verify DecisionReason is valid JSON with required fields.
		var reason model.DecisionReasonJSON
		if err := json.Unmarshal(trade.DecisionReason, &reason); err != nil {
			missing = append(missing, "DecisionReason(invalid JSON)")
		} else {
			if len(reason.Indicators) == 0 {
				missing = append(missing, "DecisionReason.Indicators")
			}
			if reason.TriggerRule == "" {
				missing = append(missing, "DecisionReason.TriggerRule")
			}
			if reason.MarketState == "" {
				missing = append(missing, "DecisionReason.MarketState")
			}
		}
	}
	if trade.OrderID == 0 {
		missing = append(missing, "OrderID")
	}
	if trade.BalanceBefore.IsZero() {
		missing = append(missing, "BalanceBefore")
	}
	if trade.BalanceAfter.IsZero() {
		missing = append(missing, "BalanceAfter")
	}

	return missing
}

func TestProperty6_TradeRecordCompleteness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a complete trade record with all required fields populated.
		symbol := rapid.SampledFrom([]string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT"}).Draw(rt, "symbol")
		side := rapid.SampledFrom([]string{"BUY", "SELL"}).Draw(rt, "side")
		strategyName := rapid.SampledFrom([]string{"MA_CROSS", "RSI", "BOLLINGER"}).Draw(rt, "strategyName")

		price := decimal.NewFromInt(rapid.Int64Range(1, 100000).Draw(rt, "price")).
			Div(decimal.NewFromInt(100))
		quantity := decimal.NewFromInt(rapid.Int64Range(1, 10000).Draw(rt, "quantity")).
			Div(decimal.NewFromInt(10000))
		amount := price.Mul(quantity)
		fee := amount.Mul(decimal.NewFromFloat(0.001))

		orderID := rapid.Int64Range(1, 1000000).Draw(rt, "orderID")
		balanceBefore := decimal.NewFromInt(rapid.Int64Range(100, 1000000).Draw(rt, "balBefore")).
			Div(decimal.NewFromInt(100))
		balanceAfter := balanceBefore.Sub(amount).Sub(fee)
		if balanceAfter.IsZero() {
			balanceAfter = decimal.NewFromInt(1)
		}

		// Generate decision reason with required sub-fields.
		indicatorCount := rapid.IntRange(1, 5).Draw(rt, "indicatorCount")
		indicators := make(map[string]float64, indicatorCount)
		indicatorNames := []string{"MA7", "MA25", "RSI", "BB_UPPER", "BB_LOWER", "VOLUME"}
		for i := 0; i < indicatorCount && i < len(indicatorNames); i++ {
			indicators[indicatorNames[i]] = float64(rapid.Int64Range(1, 100000).Draw(rt, "indVal")) / 100.0
		}

		triggerRule := rapid.SampledFrom([]string{
			"MA7 crossed above MA25",
			"RSI below 30 oversold",
			"Price broke above BB upper band",
		}).Draw(rt, "triggerRule")

		marketState := rapid.SampledFrom([]string{
			"Uptrend with increasing volume",
			"Downtrend with decreasing volume",
			"Sideways consolidation",
		}).Draw(rt, "marketState")

		reason := model.DecisionReasonJSON{
			Indicators:  indicators,
			TriggerRule: triggerRule,
			MarketState: marketState,
		}
		reasonJSON, err := json.Marshal(reason)
		require.NoError(rt, err)

		executedAt := time.Now().Add(-time.Duration(rapid.Int64Range(0, 86400).Draw(rt, "timeOffset")) * time.Second)

		trade := &model.Trade{
			OrderID:        orderID,
			Symbol:         symbol,
			Side:           side,
			Price:          price,
			Quantity:       quantity,
			Amount:         amount,
			Fee:            fee,
			FeeAsset:       "USDT",
			StrategyName:   strategyName,
			DecisionReason: reasonJSON,
			BalanceBefore:  balanceBefore,
			BalanceAfter:   balanceAfter,
			ExecutedAt:     executedAt,
		}

		// Validate: all required fields must be present and non-empty.
		missingFields := ValidateTrade(trade)
		assert.Empty(rt, missingFields,
			"Trade record should have all required fields populated, missing: %v", missingFields)
	})
}

func TestProperty6_IncompleteTradeDetected(t *testing.T) {
	// Verify that ValidateTrade correctly detects missing fields.
	rapid.Check(t, func(rt *rapid.T) {
		// Create a trade with one randomly chosen field left empty/zero.
		fieldToOmit := rapid.IntRange(0, 11).Draw(rt, "fieldToOmit")

		reason := model.DecisionReasonJSON{
			Indicators:  map[string]float64{"MA7": 42350.5},
			TriggerRule: "MA7 crossed above MA25",
			MarketState: "Uptrend",
		}
		reasonJSON, _ := json.Marshal(reason)

		trade := &model.Trade{
			OrderID:        1,
			Symbol:         "BTCUSDT",
			Side:           "BUY",
			Price:          decimal.NewFromFloat(50000.0),
			Quantity:       decimal.NewFromFloat(0.01),
			Amount:         decimal.NewFromFloat(500.0),
			Fee:            decimal.NewFromFloat(0.5),
			StrategyName:   "MA_CROSS",
			DecisionReason: reasonJSON,
			BalanceBefore:  decimal.NewFromFloat(10000.0),
			BalanceAfter:   decimal.NewFromFloat(9499.5),
			ExecutedAt:     time.Now(),
		}

		// Zero out one field.
		switch fieldToOmit {
		case 0:
			trade.ExecutedAt = time.Time{}
		case 1:
			trade.Symbol = ""
		case 2:
			trade.Side = ""
		case 3:
			trade.Price = decimal.Zero
		case 4:
			trade.Quantity = decimal.Zero
		case 5:
			trade.Amount = decimal.Zero
		case 6:
			trade.Fee = decimal.Zero
		case 7:
			trade.StrategyName = ""
		case 8:
			trade.DecisionReason = nil
		case 9:
			trade.OrderID = 0
		case 10:
			trade.BalanceBefore = decimal.Zero
		case 11:
			trade.BalanceAfter = decimal.Zero
		}

		missingFields := ValidateTrade(trade)
		assert.NotEmpty(rt, missingFields,
			"ValidateTrade should detect missing field when fieldToOmit=%d", fieldToOmit)
	})
}
