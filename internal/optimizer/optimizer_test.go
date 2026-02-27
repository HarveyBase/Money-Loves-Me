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

// mockKlineProvider 返回预配置的 K 线数据。
type mockKlineProvider struct {
	klines []binance.Kline
}

func (m *mockKlineProvider) GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error) {
	return m.klines, nil
}

// mockStrategy 用于优化器测试的模拟策略。
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
// 对于任意两组参数的回测结果，优化器应选择
// 净利润更高的参数集。
// **Validates: Requirements 11.4**
func TestProperty22_OptimizerNetProfitObjective(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成两个随机净利润
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

		// 如果两者都为负，则不应选择任何候选参数
		if profit1.IsNegative() && profit2.IsNegative() {
			assert.Nil(t, best, "no candidate should be selected when all have negative net profit")
			return
		}

		// 否则，应选择净利润更高的那个
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
// 对于任何参数优化，每个参数的变化比率不得超过 30%。
// **Validates: Requirements 11.7**
func TestProperty23_OptimizerParamChangeLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机原始参数
		numParams := rapid.IntRange(1, 5).Draw(t, "numParams")
		original := make(strategy.StrategyParams)
		proposed := make(strategy.StrategyParams)

		for i := 0; i < numParams; i++ {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "paramName")
			origVal := decimal.NewFromFloat(rapid.Float64Range(1, 100).Draw(t, "origVal"))
			// 提议一个可能超过 30% 限制的值
			propVal := decimal.NewFromFloat(rapid.Float64Range(0.1, 200).Draw(t, "propVal"))
			original[name] = origVal
			proposed[name] = propVal
		}

		opt := NewStrategyOptimizer(nil, nil, DefaultOptimizerConfig())
		clamped := opt.ClampParams(original, proposed)

		// 验证每个参数的变化比率 <= 30%
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
// 当最佳候选参数具有正净利润且优于当前参数时，应更新参数。
// 当最佳候选参数具有负净利润时，应保留当前参数。
// **Validates: Requirements 11.5, 11.8**
func TestProperty24_OptimizerDecisionCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		currentProfit := decimal.NewFromFloat(rapid.Float64Range(-500, 500).Draw(t, "currentProfit"))
		bestProfit := decimal.NewFromFloat(rapid.Float64Range(-500, 500).Draw(t, "bestProfit"))

		currentResult := &backtest.BacktestResult{NetProfit: currentProfit}
		bestResult := &backtest.BacktestResult{NetProfit: bestProfit}

		shouldApply := ShouldApply(currentResult, bestResult)

		if bestProfit.IsNegative() || bestProfit.IsZero() {
			// 负或零净利润 → 不应应用
			assert.False(t, shouldApply,
				"should not apply when best net profit (%s) is non-positive", bestProfit.String())
		} else if bestProfit.GreaterThan(currentProfit) {
			// 正且优于当前 → 应应用
			assert.True(t, shouldApply,
				"should apply when best profit (%s) > current profit (%s)",
				bestProfit.String(), currentProfit.String())
		} else {
			// 正但不优于当前 → 不应应用
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

	// 所有候选参数应具有相同的参数名称
	for _, c := range candidates {
		assert.Contains(t, c, "period")
		assert.Contains(t, c, "level")
	}
}
