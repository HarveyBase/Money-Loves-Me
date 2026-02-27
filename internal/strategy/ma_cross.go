package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	MACrossName             = "MA_CROSS"
	MACrossParamShortPeriod = "short_period"
	MACrossParamLongPeriod  = "long_period"
)

// MACrossStrategy implements a moving average crossover strategy.
// A BUY signal is generated when the short MA crosses above the long MA.
// A SELL signal is generated when the short MA crosses below the long MA.
type MACrossStrategy struct {
	shortPeriod int
	longPeriod  int
}

// NewMACrossStrategy creates a new MA Cross strategy with default parameters.
func NewMACrossStrategy() *MACrossStrategy {
	return &MACrossStrategy{
		shortPeriod: 7,
		longPeriod:  25,
	}
}

// Name returns the strategy name.
func (s *MACrossStrategy) Name() string {
	return MACrossName
}

// Calculate analyzes klines and returns a BUY/SELL signal based on MA crossover.
// Returns nil if there is insufficient data or no crossover detected.
func (s *MACrossStrategy) Calculate(klines []binance.Kline) *Signal {
	if len(klines) < s.longPeriod+1 {
		return nil
	}

	// We need at least longPeriod+1 klines to detect a crossover
	// (current bar and previous bar MAs).
	n := len(klines)

	// Calculate short and long MAs for the current and previous bars.
	shortMACurr := calcSMA(klines[n-s.shortPeriod : n])
	shortMAPrev := calcSMA(klines[n-1-s.shortPeriod : n-1])
	longMACurr := calcSMA(klines[n-s.longPeriod : n])
	longMAPrev := calcSMA(klines[n-1-s.longPeriod : n-1])

	lastKline := klines[n-1]

	// Golden cross: short MA crosses above long MA → BUY
	if shortMAPrev.LessThanOrEqual(longMAPrev) && shortMACurr.GreaterThan(longMACurr) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					fmt.Sprintf("MA%d", s.shortPeriod): shortMACurr.InexactFloat64(),
					fmt.Sprintf("MA%d", s.longPeriod):  longMACurr.InexactFloat64(),
				},
				TriggerRule: fmt.Sprintf("MA%d crossed above MA%d (golden cross)", s.shortPeriod, s.longPeriod),
				MarketState: "bullish crossover",
			},
		}
	}

	// Death cross: short MA crosses below long MA → SELL
	if shortMAPrev.GreaterThanOrEqual(longMAPrev) && shortMACurr.LessThan(longMACurr) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					fmt.Sprintf("MA%d", s.shortPeriod): shortMACurr.InexactFloat64(),
					fmt.Sprintf("MA%d", s.longPeriod):  longMACurr.InexactFloat64(),
				},
				TriggerRule: fmt.Sprintf("MA%d crossed below MA%d (death cross)", s.shortPeriod, s.longPeriod),
				MarketState: "bearish crossover",
			},
		}
	}

	return nil
}

// GetParams returns the current strategy parameters.
func (s *MACrossStrategy) GetParams() StrategyParams {
	return StrategyParams{
		MACrossParamShortPeriod: decimal.NewFromInt(int64(s.shortPeriod)),
		MACrossParamLongPeriod:  decimal.NewFromInt(int64(s.longPeriod)),
	}
}

// SetParams updates the strategy parameters.
func (s *MACrossStrategy) SetParams(params StrategyParams) error {
	sp, ok := params[MACrossParamShortPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", MACrossParamShortPeriod)
	}
	lp, ok := params[MACrossParamLongPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", MACrossParamLongPeriod)
	}

	shortVal := int(sp.IntPart())
	longVal := int(lp.IntPart())

	if shortVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", MACrossParamShortPeriod, shortVal)
	}
	if longVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", MACrossParamLongPeriod, longVal)
	}
	if shortVal >= longVal {
		return fmt.Errorf("short_period (%d) must be less than long_period (%d)", shortVal, longVal)
	}

	s.shortPeriod = shortVal
	s.longPeriod = longVal
	return nil
}

// EstimateFee estimates the trading fee for a given price, quantity, and fee rate.
func (s *MACrossStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcSMA calculates the simple moving average of the Close prices of the given klines.
func calcSMA(klines []binance.Kline) decimal.Decimal {
	if len(klines) == 0 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, k := range klines {
		sum = sum.Add(k.Close)
	}
	return sum.Div(decimal.NewFromInt(int64(len(klines))))
}
