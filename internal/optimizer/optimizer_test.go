package optimizer

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"money-loves-me/internal/backtest"
	"money-loves-me/internal/strategy"
	"money-loves-me/pkg/binance"
)

// mockKlineProvider returns pre-configured klines.
type mockKlineProvider struct {
	klines []binance.Kline
}

func (m *mockKlineProvider) GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error) {
	return m.klines, nil
}

// mockStrategy for optimizer testing.
type mockStrategy struct {
	name   string
	params strategy.StrategyParams
}

func (m *mockStrategy) Name() string { return m.name }
func (m *mockStrategy) Calculate(klines []binance.Kline) *strategy.Signal {
	return nil
}
func (m *mockStrategy) GetParams() strategy.StrategyParams {
	result := make(strategy.StrategyParams)
	for k, v := range m.params {
		result[k] = v
	}
	return result
}
func (m *mockStrategy) SetParams(p strategy.StrategyParams) error {
	m.params = make(strategy.StrategyParams)
	for k, v := range p {
		m.params[k] = v
	}
	return nil
}
func (m *mockStrategy) EstimateFee(price, qty, rate decimal.Decimal) decimal.Decimal {
	return price.Mul(qty).Mul(rate)
}

// Feature: binance-trading-system, Property 22: 优化器以净收益率为目标
// For any two sets of parameters' backtest results, the optimizer should select
// the parameter set with higher net profit rate.
// **Validates: Requirements 11.4**
func TestProperty22_OptimizerNetProfitObjective(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two random net profits
		profit1 := decimal.NewFromFloat(rapid.Float64Range(-1000, 1000).Draw(t, "profit1"))
		profit2 := decimal.NewFromFloat(rapid.Float64Range(-1000, 1000).Draw(t, "profit2"))

		candidates := []CandidateResult{
			{
				Params: strategy.StrategyParams{"a": decimal.NewFromInt(1)},
				Result: &backtest.BacktestResult{NetProfit: profit1},
			},
			{
				Params: strategy.StrategyParams{"a": decimal.NewFromInt(2)},
				Result: &backtest.BacktestResult{NetProfit: profit2},
			},
		}

		best := SelectBest(candidates)

		// If both are negative, no best should be selected
		if profit1.IsNegative() && profit2.IsNegative() {
			assert.Nil(t, best, "no candidate should be selected when all have negative net profit")
			return
		}

		// Otherwise, the one with higher net profit should be selected
		if best != nil {
			higherProfit := profit1
			if profit2.GreaterThan(profit1) {
				higherProfit = profit2
			}
			assert.True(t, best.Result.NetProfit.Equal(higherProfit),
				"selected net profit (%s) should be the higher one (%s)",
				best.Result.NetProfit.String(), higherProfit.String())
		}
	})
}

// Feature: binance-trading-system, Property 23: 优化器参数变化幅度限制
// For any parameter optimization, each parameter's change ratio must not exceed 30%.
// **Validates: Requirements 11.7**
func TestProperty23_OptimizerParamChangeLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random original parameters
		numParams := rapid.IntRange(1, 5).Draw(t, "numParams")
		original := make(strategy.StrategyParams)
		proposed := make(strategy.StrategyParams)

		for i := 0; i < numParams; i++ {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "paramName")
			origVal := decimal.NewFromFloat(rapid.Float64Range(1, 100).Draw(t, "origVal"))
			// Propose a value that may exceed the 30% limit
			propVal := decimal.NewFromFloat(rapid.Float64Range(0.1, 200).Draw(t, "propVal"))
			original[name] = origVal
			proposed[name] = propVal
		}

		opt := NewStrategyOptimizer(nil, nil, DefaultOptimizerConfig())
		clamped := opt.ClampParams(original, proposed)

		// Verify each parameter's change ratio <= 30%
		for name, origVal := range original {
			clampedVal := clamped[name]
			if origVal.IsZero() {
				continue
			}
			changeRatio := clampedVal.Sub(origVal).Div(origVal).Abs().InexactFloat64()
			assert.LessOrEqual(t, changeRatio, 0.3+1e-9,
				"parameter %s change ratio (%.6f) should not exceed 30%% (orig=%s, new=%s)",
				name, changeRatio, origVal.String(), clampedVal.String())
		}
	})
}

// Feature: binance-trading-system, Property 24: 优化器决策正确性
// When the best candidate has positive net profit and outperforms current, params should be updated.
// When the best candidate has negative net profit, current params should be kept.
// **Validates: Requirements 11.5, 11.8**
func TestProperty24_OptimizerDecisionCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		currentProfit := decimal.NewFromFloat(rapid.Float64Range(-500, 500).Draw(t, "currentProfit"))
		bestProfit := decimal.NewFromFloat(rapid.Float64Range(-500, 500).Draw(t, "bestProfit"))

		currentResult := &backtest.BacktestResult{NetProfit: currentProfit}
		bestResult := &backtest.BacktestResult{NetProfit: bestProfit}

		shouldApply := ShouldApply(currentResult, bestResult)

		if bestProfit.IsNegative() || bestProfit.IsZero() {
			// Negative or zero net profit → should NOT apply
			assert.False(t, shouldApply,
				"should not apply when best net profit (%s) is non-positive", bestProfit.String())
		} else if bestProfit.GreaterThan(currentProfit) {
			// Positive and better than current → should apply
			assert.True(t, shouldApply,
				"should apply when best profit (%s) > current profit (%s)",
				bestProfit.String(), currentProfit.String())
		} else {
			// Positive but not better → should NOT apply
			assert.False(t, shouldApply,
				"should not apply when best profit (%s) <= current profit (%s)",
				bestProfit.String(), currentProfit.String())
		}
	})
}

func TestParamChangeRatio(t *testing.T) {
	old := strategy.StrategyParams{
		"period": decimal.NewFromInt(10),
		"mult":   decimal.NewFromFloat(2.0),
	}
	new := strategy.StrategyParams{
		"period": decimal.NewFromInt(12),
		"mult":   decimal.NewFromFloat(2.5),
	}

	ratios := ParamChangeRatio(old, new)
	assert.InDelta(t, 0.2, ratios["period"], 0.001)
	assert.InDelta(t, 0.25, ratios["mult"], 0.001)
}

func TestGenerateCandidates(t *testing.T) {
	opt := NewStrategyOptimizer(nil, nil, DefaultOptimizerConfig())
	params := strategy.StrategyParams{
		"period": decimal.NewFromInt(14),
		"level":  decimal.NewFromFloat(70),
	}

	candidates := opt.GenerateCandidates(params, 5)
	assert.Len(t, candidates, 5)

	// All candidates should have the same parameter names
	for _, c := range candidates {
		assert.Contains(t, c, "period")
		assert.Contains(t, c, "level")
	}
}
