package strategy

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"money-loves-me/pkg/binance"
)

// generateKlines creates n klines starting from a base price with small increments.
func generateKlines(n int, basePrice decimal.Decimal) []binance.Kline {
	klines := make([]binance.Kline, n)
	for i := 0; i < n; i++ {
		price := basePrice.Add(decimal.NewFromInt(int64(i)))
		klines[i] = binance.Kline{
			OpenTime:  time.Now().Add(time.Duration(-n+i) * time.Minute),
			Open:      price,
			High:      price.Add(decimal.NewFromFloat(1)),
			Low:       price.Sub(decimal.NewFromFloat(1)),
			Close:     price,
			Volume:    decimal.NewFromFloat(100),
			CloseTime: time.Now().Add(time.Duration(-n+i+1) * time.Minute),
		}
	}
	return klines
}

// Feature: binance-trading-system, Property 7: 策略自动初始化有效默认参数
// For any built-in strategy, GetParams() should return a valid default parameter set
// where all parameter values are positive.
// **Validates: Requirements 4.2**
func TestProperty7_StrategyDefaultParamsAreValid(t *testing.T) {
	builtInFactories := []func() Strategy{
		func() Strategy { return NewMACrossStrategy() },
		func() Strategy { return NewRSIStrategy() },
		func() Strategy { return NewBollingerStrategy() },
	}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random built-in strategy
		idx := rapid.IntRange(0, len(builtInFactories)-1).Draw(t, "strategyIndex")
		strategy := builtInFactories[idx]()

		params := strategy.GetParams()

		// All parameters must be present (non-empty map)
		assert.NotEmpty(t, params, "strategy %s should have default parameters", strategy.Name())

		// All parameter values must be positive
		for name, value := range params {
			assert.True(t, value.GreaterThan(decimal.Zero),
				"strategy %s parameter %s should be positive, got %s", strategy.Name(), name, value.String())
		}

		// Verify SetParams accepts the default params (they should be valid)
		err := strategy.SetParams(params)
		assert.NoError(t, err, "strategy %s should accept its own default params", strategy.Name())
	})
}

// --- Unit tests for StrategyEngine core logic ---

func TestStrategyEngine_StartStop(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	// Should not be running initially
	assert.False(t, engine.IsRunning())

	// Start
	err := engine.Start(context.Background())
	assert.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// Double start should error
	err = engine.Start(context.Background())
	assert.Error(t, err)

	// Stop
	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.IsRunning())

	// Double stop should error
	err = engine.Stop()
	assert.Error(t, err)
}

func TestStrategyEngine_EvaluateMarket_NotRunning(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	// Should return nil when not running
	signals := engine.EvaluateMarket("BTCUSDT", nil)
	assert.Nil(t, signals)
}

func TestStrategyEngine_StrategyLogs(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	err := engine.Start(context.Background())
	assert.NoError(t, err)
	defer engine.Stop()

	// Evaluate with insufficient data - should log but no signals
	klines := generateKlines(5, decimal.NewFromFloat(100))
	signals := engine.EvaluateMarket("BTCUSDT", klines)
	assert.Empty(t, signals)

	// Check that logs were created
	logs := engine.GetStrategyLogs(MACrossName)
	assert.NotEmpty(t, logs)
	assert.Equal(t, MACrossName, logs[0].StrategyName)
	assert.Equal(t, "BTCUSDT", logs[0].Symbol)
}

// Feature: binance-trading-system, Property 8: 停止交易后不产生新信号
// After calling Stop(), no matter how many market data updates are received,
// no new trading signals should be generated.
// **Validates: Requirements 4.6**
func TestProperty8_NoSignalsAfterStop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random number of market updates to send after stop
		numUpdates := rapid.IntRange(1, 50).Draw(t, "numUpdates")
		numKlines := rapid.IntRange(30, 100).Draw(t, "numKlines")

		// Track signals received
		var signalsMu sync.Mutex
		var receivedSignals []Signal

		handler := SignalHandlerFunc(func(signal Signal) error {
			signalsMu.Lock()
			receivedSignals = append(receivedSignals, signal)
			signalsMu.Unlock()
			return nil
		})

		engine := NewStrategyEngine(
			[]Strategy{NewMACrossStrategy(), NewRSIStrategy(), NewBollingerStrategy()},
			handler,
			FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
		)

		// Start and then immediately stop
		err := engine.Start(context.Background())
		assert.NoError(t, err)

		err = engine.Stop()
		assert.NoError(t, err)
		assert.False(t, engine.IsRunning())

		// Now send many market data updates - no signals should be generated
		for i := 0; i < numUpdates; i++ {
			basePrice := decimal.NewFromFloat(100).Add(decimal.NewFromInt(int64(i)))
			klines := generateKlines(numKlines, basePrice)
			signals := engine.EvaluateMarket("BTCUSDT", klines)
			assert.Empty(t, signals, "no signals should be generated after Stop()")
		}

		// Also verify ProcessSignals produces nothing
		klines := generateKlines(numKlines, decimal.NewFromFloat(200))
		err = engine.ProcessSignals("BTCUSDT", klines)
		assert.NoError(t, err)

		signalsMu.Lock()
		assert.Empty(t, receivedSignals, "signal handler should not receive any signals after Stop()")
		signalsMu.Unlock()
	})
}

// mockStrategy is a test strategy that always generates a signal with configurable parameters.
type mockStrategy struct {
	name   string
	signal *Signal
	params StrategyParams
}

func (m *mockStrategy) Name() string { return m.name }

func (m *mockStrategy) Calculate(klines []binance.Kline) *Signal {
	if m.signal == nil {
		return nil
	}
	// Return a copy
	s := *m.signal
	return &s
}

func (m *mockStrategy) GetParams() StrategyParams { return m.params }

func (m *mockStrategy) SetParams(params StrategyParams) error {
	m.params = params
	return nil
}

func (m *mockStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// Feature: binance-trading-system, Property 9: 手续费感知的信号生成
// For any trading signal, the expected profit after deducting fees must be positive;
// otherwise the signal should not be generated.
// **Validates: Requirements 4.8**
func TestProperty9_FeeAwareSignalGeneration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random price, quantity, expected profit, and fee rate
		price := decimal.NewFromFloat(rapid.Float64Range(10, 100000).Draw(t, "price"))
		quantity := decimal.NewFromFloat(rapid.Float64Range(0.001, 100).Draw(t, "quantity"))
		feeRateFloat := rapid.Float64Range(0.0001, 0.01).Draw(t, "feeRate")
		feeRate := decimal.NewFromFloat(feeRateFloat)

		// Calculate the fee for this trade
		fee := price.Mul(quantity).Mul(feeRate)

		// Randomly decide if the expected profit should be above or below the fee
		profitAboveFee := rapid.Bool().Draw(t, "profitAboveFee")

		var expectedProfit decimal.Decimal
		if profitAboveFee {
			// Expected profit > fee → signal should pass
			margin := decimal.NewFromFloat(rapid.Float64Range(0.01, 100).Draw(t, "margin"))
			expectedProfit = fee.Add(margin)
		} else {
			// Expected profit <= fee → signal should be rejected
			// Generate a profit that is between 0 and fee (inclusive of fee)
			ratio := decimal.NewFromFloat(rapid.Float64Range(0, 1).Draw(t, "ratio"))
			expectedProfit = fee.Mul(ratio)
		}

		signal := &Signal{
			Strategy:       "TEST_STRATEGY",
			Symbol:         "BTCUSDT",
			Side:           SignalBuy,
			Price:          price,
			Quantity:       quantity,
			ExpectedProfit: expectedProfit,
			Timestamp:      time.Now(),
			Reason: SignalReason{
				Indicators:  map[string]float64{"test": 1.0},
				TriggerRule: "test rule",
				MarketState: "test state",
			},
		}

		mock := &mockStrategy{
			name:   "TEST_STRATEGY",
			signal: signal,
			params: StrategyParams{"test": decimal.NewFromInt(1)},
		}

		engine := NewStrategyEngine(
			[]Strategy{mock},
			nil,
			FeeRate{Maker: feeRate, Taker: feeRate},
		)

		err := engine.Start(context.Background())
		assert.NoError(t, err)
		defer engine.Stop()

		klines := generateKlines(5, price)
		signals := engine.EvaluateMarket("BTCUSDT", klines)

		if profitAboveFee {
			// Signal should be generated (expected profit > fee)
			assert.Len(t, signals, 1, "signal should be generated when expected profit > fee")
			if len(signals) > 0 {
				// Verify the signal's expected profit exceeds the fee
				actualFee := mock.EstimateFee(signals[0].Price, signals[0].Quantity, feeRate)
				assert.True(t, signals[0].ExpectedProfit.GreaterThan(actualFee),
					"expected profit (%s) should exceed fee (%s)",
					signals[0].ExpectedProfit.String(), actualFee.String())
			}
		} else {
			// Signal should NOT be generated (expected profit <= fee)
			assert.Empty(t, signals, "signal should not be generated when expected profit <= fee")
		}
	})
}
