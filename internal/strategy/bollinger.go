package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	BollingerName            = "BOLLINGER"
	BollingerParamPeriod     = "period"
	BollingerParamStdDevMult = "std_dev_multiplier"
)

// BollingerStrategy implements a Bollinger Bands breakout strategy.
// A BUY signal is generated when the price touches or breaks below the lower band.
// A SELL signal is generated when the price touches or breaks above the upper band.
type BollingerStrategy struct {
	period     int
	stdDevMult decimal.Decimal
}

// NewBollingerStrategy creates a new Bollinger Bands strategy with default parameters.
func NewBollingerStrategy() *BollingerStrategy {
	return &BollingerStrategy{
		period:     20,
		stdDevMult: decimal.NewFromFloat(2.0),
	}
}

// Name returns the strategy name.
func (s *BollingerStrategy) Name() string {
	return BollingerName
}

// Calculate analyzes klines and returns a BUY/SELL signal based on Bollinger Bands.
// Returns nil if there is insufficient data or price is within the bands.
func (s *BollingerStrategy) Calculate(klines []binance.Kline) *Signal {
	if len(klines) < s.period {
		return nil
	}

	n := len(klines)
	recent := klines[n-s.period:]

	middle := calcSMA(recent)
	stdDev := s.calcStdDev(recent, middle)

	upper := middle.Add(s.stdDevMult.Mul(stdDev))
	lower := middle.Sub(s.stdDevMult.Mul(stdDev))

	lastKline := klines[n-1]
	price := lastKline.Close

	indicators := map[string]float64{
		"BB_Upper":  upper.InexactFloat64(),
		"BB_Middle": middle.InexactFloat64(),
		"BB_Lower":  lower.InexactFloat64(),
		"BB_StdDev": stdDev.InexactFloat64(),
	}

	// Price at or below lower band → BUY
	if price.LessThanOrEqual(lower) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     price,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators:  indicators,
				TriggerRule: fmt.Sprintf("Price (%.2f) <= lower band (%.2f)", price.InexactFloat64(), lower.InexactFloat64()),
				MarketState: "oversold - price at lower Bollinger Band",
			},
		}
	}

	// Price at or above upper band → SELL
	if price.GreaterThanOrEqual(upper) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     price,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators:  indicators,
				TriggerRule: fmt.Sprintf("Price (%.2f) >= upper band (%.2f)", price.InexactFloat64(), upper.InexactFloat64()),
				MarketState: "overbought - price at upper Bollinger Band",
			},
		}
	}

	return nil
}

// GetParams returns the current strategy parameters.
func (s *BollingerStrategy) GetParams() StrategyParams {
	return StrategyParams{
		BollingerParamPeriod:     decimal.NewFromInt(int64(s.period)),
		BollingerParamStdDevMult: s.stdDevMult,
	}
}

// SetParams updates the strategy parameters.
func (s *BollingerStrategy) SetParams(params StrategyParams) error {
	p, ok := params[BollingerParamPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", BollingerParamPeriod)
	}
	m, ok := params[BollingerParamStdDevMult]
	if !ok {
		return fmt.Errorf("missing parameter: %s", BollingerParamStdDevMult)
	}

	periodVal := int(p.IntPart())
	if periodVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", BollingerParamPeriod, periodVal)
	}
	if m.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%s must be positive, got %s", BollingerParamStdDevMult, m.String())
	}

	s.period = periodVal
	s.stdDevMult = m
	return nil
}

// EstimateFee estimates the trading fee for a given price, quantity, and fee rate.
func (s *BollingerStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcStdDev computes the population standard deviation of Close prices.
func (s *BollingerStrategy) calcStdDev(klines []binance.Kline, mean decimal.Decimal) decimal.Decimal {
	if len(klines) == 0 {
		return decimal.Zero
	}

	sumSquares := decimal.Zero
	for _, k := range klines {
		diff := k.Close.Sub(mean)
		sumSquares = sumSquares.Add(diff.Mul(diff))
	}

	variance := sumSquares.Div(decimal.NewFromInt(int64(len(klines))))
	return decimalSqrt(variance)
}

// decimalSqrt computes the square root of a decimal using Newton's method.
func decimalSqrt(d decimal.Decimal) decimal.Decimal {
	if d.IsZero() || d.IsNegative() {
		return decimal.Zero
	}

	// Newton's method: x_{n+1} = (x_n + d/x_n) / 2
	two := decimal.NewFromInt(2)
	x := d // initial guess
	for i := 0; i < 50; i++ {
		next := x.Add(d.Div(x)).Div(two)
		// Check convergence with high precision.
		if next.Sub(x).Abs().LessThan(decimal.New(1, -16)) {
			return next
		}
		x = next
	}
	return x
}
