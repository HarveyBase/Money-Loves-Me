package optimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"money-loves-me/internal/backtest"
	"money-loves-me/internal/model"
	"money-loves-me/internal/strategy"

	"github.com/shopspring/decimal"
)

// OptimizerConfig 保存优化器配置。
type OptimizerConfig struct {
	Interval       time.Duration // 优化周期，默认 24 小时
	LookbackDays   int           // 回溯天数，默认 30
	MaxParamChange float64       // 最大参数变化比率，默认 0.3（30%）
}

// DefaultOptimizerConfig 返回默认的优化器配置。
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		Interval:       24 * time.Hour,
		LookbackDays:   30,
		MaxParamChange: 0.3,
	}
}

// RecordStore 抽象优化记录的持久化存储。
type RecordStore interface {
	Create(record *model.OptimizationRecord) error
	GetAll() ([]model.OptimizationRecord, error)
	GetByStrategy(strategyName string) ([]model.OptimizationRecord, error)
}

// StrategyOptimizer 使用回测来优化策略参数。
type StrategyOptimizer struct {
	backtester *backtest.Backtester
	store      RecordStore
	config     OptimizerConfig
}

// NewStrategyOptimizer 创建新的 StrategyOptimizer。
func NewStrategyOptimizer(bt *backtest.Backtester, store RecordStore, cfg OptimizerConfig) *StrategyOptimizer {
	return &StrategyOptimizer{
		backtester: bt,
		store:      store,
		config:     cfg,
	}
}

// CandidateResult 将参数集与其回测结果配对。
type CandidateResult struct {
	Params strategy.StrategyParams
	Result *backtest.BacktestResult
}

// GenerateCandidates 在当前参数的 ±MaxParamChange 范围内生成候选参数集。
func (o *StrategyOptimizer) GenerateCandidates(currentParams strategy.StrategyParams, numCandidates int) []strategy.StrategyParams {
	candidates := make([]strategy.StrategyParams, 0, numCandidates)
	maxChange := o.config.MaxParamChange

	// 在允许范围内生成均匀分布的候选参数
	for i := 0; i < numCandidates; i++ {
		candidate := make(strategy.StrategyParams)
		ratio := -maxChange + (2*maxChange)*float64(i)/float64(numCandidates)

		for name, value := range currentParams {
			change := value.Mul(decimal.NewFromFloat(ratio))
			newVal := value.Add(change)
			if newVal.LessThanOrEqual(decimal.Zero) {
				newVal = value.Mul(decimal.NewFromFloat(0.1)) // 下限为原始值的 10%
			}
			candidate[name] = newVal
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// ClampParams 确保每个参数的变化幅度不超过原始值的 MaxParamChange。
func (o *StrategyOptimizer) ClampParams(original, proposed strategy.StrategyParams) strategy.StrategyParams {
	clamped := make(strategy.StrategyParams)
	maxChange := o.config.MaxParamChange

	for name, origVal := range original {
		newVal, ok := proposed[name]
		if !ok {
			clamped[name] = origVal
			continue
		}
		if origVal.IsZero() {
			clamped[name] = newVal
			continue
		}
		changeRatio := newVal.Sub(origVal).Div(origVal).Abs().InexactFloat64()
		if changeRatio > maxChange {
			// 限制在最大允许变化范围内
			if newVal.GreaterThan(origVal) {
				clamped[name] = origVal.Mul(decimal.NewFromFloat(1 + maxChange))
			} else {
				clamped[name] = origVal.Mul(decimal.NewFromFloat(1 - maxChange))
			}
		} else {
			clamped[name] = newVal
		}
	}
	return clamped
}

// SelectBest 选择净利润最高的候选参数。
// 如果没有候选参数具有正净利润则返回 nil。
func SelectBest(candidates []CandidateResult) *CandidateResult {
	var best *CandidateResult
	for i := range candidates {
		c := &candidates[i]
		if c.Result == nil {
			continue
		}
		if best == nil || c.Result.NetProfit.GreaterThan(best.Result.NetProfit) {
			best = c
		}
	}
	if best != nil && best.Result.NetProfit.IsNegative() {
		return nil // 没有正收益的候选参数
	}
	return best
}

// ShouldApply 判断优化后的参数是否应替换当前参数。
// 当最佳候选参数具有正净利润且优于当前参数时返回 true。
func ShouldApply(currentResult, bestResult *backtest.BacktestResult) bool {
	if bestResult == nil || currentResult == nil {
		return false
	}
	// 最佳候选必须具有正净利润
	if bestResult.NetProfit.IsNegative() || bestResult.NetProfit.IsZero() {
		return false
	}
	// 最佳候选必须优于当前参数
	return bestResult.NetProfit.GreaterThan(currentResult.NetProfit)
}

// RunOptimization 为策略运行完整的优化周期。
func (o *StrategyOptimizer) RunOptimization(ctx context.Context, strat strategy.Strategy, btCfg backtest.BacktestConfig) (*CandidateResult, bool, error) {
	currentParams := strat.GetParams()

	// 使用当前参数运行回测
	currentResult, err := o.backtester.Run(btCfg)
	if err != nil {
		return nil, false, fmt.Errorf("backtest with current params failed: %w", err)
	}

	// 生成候选参数
	candidates := o.GenerateCandidates(currentParams, 10)
	var candidateResults []CandidateResult

	for _, params := range candidates {
		clamped := o.ClampParams(currentParams, params)
		if err := strat.SetParams(clamped); err != nil {
			continue
		}
		result, err := o.backtester.Run(btCfg)
		if err != nil {
			continue
		}
		candidateResults = append(candidateResults, CandidateResult{Params: clamped, Result: result})
	}

	// 恢复原始参数
	strat.SetParams(currentParams)

	best := SelectBest(candidateResults)
	applied := false

	if best != nil && ShouldApply(currentResult, best.Result) {
		strat.SetParams(best.Params)
		applied = true
	}

	// 保存优化记录
	if o.store != nil {
		o.saveRecord(strat.Name(), currentParams, best, currentResult, applied)
	}

	if best != nil {
		return best, applied, nil
	}
	return nil, false, nil
}

func (o *StrategyOptimizer) saveRecord(stratName string, oldParams strategy.StrategyParams, best *CandidateResult, currentResult *backtest.BacktestResult, applied bool) {
	oldParamsJSON, _ := json.Marshal(oldParams)
	oldMetricsJSON, _ := json.Marshal(currentResult)

	var newParamsJSON, newMetricsJSON []byte
	notes := "No better candidate found"
	if best != nil {
		newParamsJSON, _ = json.Marshal(best.Params)
		newMetricsJSON, _ = json.Marshal(best.Result)
		if applied {
			notes = "Parameters updated: better candidate found with positive net profit"
		} else {
			notes = fmt.Sprintf("Parameters kept: best candidate net profit = %s", best.Result.NetProfit.String())
		}
	} else {
		newParamsJSON = oldParamsJSON
		newMetricsJSON = oldMetricsJSON
	}

	record := &model.OptimizationRecord{
		StrategyName:  stratName,
		OldParams:     oldParamsJSON,
		NewParams:     newParamsJSON,
		OldMetrics:    oldMetricsJSON,
		NewMetrics:    newMetricsJSON,
		AnalysisNotes: &notes,
		Applied:       applied,
	}
	o.store.Create(record)
}

// GetHistory 获取所有优化记录。
func (o *StrategyOptimizer) GetHistory() ([]model.OptimizationRecord, error) {
	if o.store == nil {
		return nil, nil
	}
	return o.store.GetAll()
}

// ParamChangeRatio 计算每个参数的变化比率。
func ParamChangeRatio(oldParams, newParams strategy.StrategyParams) map[string]float64 {
	ratios := make(map[string]float64)
	for name, oldVal := range oldParams {
		newVal, ok := newParams[name]
		if !ok || oldVal.IsZero() {
			ratios[name] = 0
			continue
		}
		ratios[name] = math.Abs(newVal.Sub(oldVal).Div(oldVal).InexactFloat64())
	}
	return ratios
}
