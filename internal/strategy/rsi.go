package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	RSIName            = "RSI"
	RSIParamPeriod     = "period"
	RSIParamOverbought = "overbought"
	RSIParamOversold   = "oversold"
)

// RSIStrategy implements an RSI overbought/oversold strategy.
// A BUY signal is generated when RSI crosses below the oversold level.
// A SELL signal is generated when RSI crosses above the overbought level.
type RSIStrategy struct {
	period     int
	overbought decimal.Decimal
	oversold   decimal.Decimal
}

// NewRSIStrategy creates a new RSI strategy with default parameters.
func NewRSIStrategy() *RSIStrategy {
	return &RSIStrategy{
		period:     14,
		overbought: decimal.NewFromInt(70),
		oversold:   decimal.NewFromInt(30),
	}
}

// Name returns the strategy name.
func (s *RSIStrategy) Name() string {
	return RSIName
}

// Calculate analyzes klines and returns a BUY/SELL signal based on RSI levels.
// Returns nil if there is insufficient data or RSI is in the neutral zone.
func (s *RSIStrategy) Calculate(klines []binance.Kline) *Signal {
	// Need at least period+1 klines to compute RSI (period price changes).
	if len(klines) < s.period+1 {
		return nil
	}

	rsi := s.calcRSI(klines)
	if rsi.IsNegative() {
		return nil // not enough data or division issue
	}

	lastKline := klines[len(klines)-1]
	rsiFloat := rsi.InexactFloat64()

	// Oversold → BUY signal
	if rsi.LessThanOrEqual(s.oversold) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					"RSI": rsiFloat,
				},
				TriggerRule: fmt.Sprintf("RSI (%.2f) <= oversold level (%.2f)", rsiFloat, s.oversold.InexactFloat64()),
				MarketState: "oversold",
			},
		}
	}

	// Overbought → SELL signal
	if rsi.GreaterThanOrEqual(s.overbought) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					"RSI": rsiFloat,
				},
				TriggerRule: fmt.Sprintf("RSI (%.2f) >= overbought level (%.2f)", rsiFloat, s.overbought.InexactFloat64()),
				MarketState: "overbought",
			},
		}
	}

	return nil
}

// GetParams returns the current strategy parameters.
func (s *RSIStrategy) GetParams() StrategyParams {
	return StrategyParams{
		RSIParamPeriod:     decimal.NewFromInt(int64(s.period)),
		RSIParamOverbought: s.overbought,
		RSIParamOversold:   s.oversold,
	}
}

// SetParams updates the strategy parameters.
func (s *RSIStrategy) SetParams(params StrategyParams) error {
	p, ok := params[RSIParamPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamPeriod)
	}
	ob, ok := params[RSIParamOverbought]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamOverbought)
	}
	os, ok := params[RSIParamOversold]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamOversold)
	}

	periodVal := int(p.IntPart())
	if periodVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", RSIParamPeriod, periodVal)
	}
	if ob.LessThanOrEqual(decimal.Zero) || ob.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("%s must be between 0 and 100, got %s", RSIParamOverbought, ob.String())
	}
	if os.LessThan(decimal.Zero) || os.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return fmt.Errorf("%s must be between 0 and 100, got %s", RSIParamOversold, os.String())
	}
	if os.GreaterThanOrEqual(ob) {
		return fmt.Errorf("oversold (%s) must be less than overbought (%s)", os.String(), ob.String())
	}

	s.period = periodVal
	s.overbought = ob
	s.oversold = os
	return nil
}

// EstimateFee estimates the trading fee for a given price, quantity, and fee rate.
func (s *RSIStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcRSI computes the RSI value using the last period+1 klines.
// Uses the standard RSI formula with simple average of gains and losses.
func (s *RSIStrategy) calcRSI(klines []binance.Kline) decimal.Decimal {
	n := len(klines)
	if n < s.period+1 {
		return decimal.NewFromInt(-1)
	}

	// Use the most recent period+1 klines.
	recent := klines[n-s.period-1:]

	gains := decimal.Zero
	losses := decimal.Zero

	for i := 1; i < len(recent); i++ {
		change := recent[i].Close.Sub(recent[i-1].Close)
		if change.IsPositive() {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Abs())
		}
	}

	periodDec := decimal.NewFromInt(int64(s.period))
	avgGain := gains.Div(periodDec)
	avgLoss := losses.Div(periodDec)

	if avgLoss.IsZero() {
		return decimal.NewFromInt(100) // no losses → RSI = 100
	}
	if avgGain.IsZero() {
		return decimal.Zero // no gains → RSI = 0
	}

	rs := avgGain.Div(avgLoss)
	rsi := decimal.NewFromInt(100).Sub(
		decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)),
	)
	return rsi
}
