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

// OptimizerConfig holds the optimizer configuration.
type OptimizerConfig struct {
	Interval       time.Duration // optimization cycle, default 24h
	LookbackDays   int           // lookback period in days, default 30
	MaxParamChange float64       // max parameter change ratio, default 0.3 (30%)
}

// DefaultOptimizerConfig returns the default optimizer configuration.
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		Interval:       24 * time.Hour,
		LookbackDays:   30,
		MaxParamChange: 0.3,
	}
}

// RecordStore abstracts persistence of optimization records.
type RecordStore interface {
	Create(record *model.OptimizationRecord) error
	GetAll() ([]model.OptimizationRecord, error)
	GetByStrategy(strategyName string) ([]model.OptimizationRecord, error)
}

// StrategyOptimizer optimizes strategy parameters using backtesting.
type StrategyOptimizer struct {
	backtester *backtest.Backtester
	store      RecordStore
	config     OptimizerConfig
}

// NewStrategyOptimizer creates a new StrategyOptimizer.
func NewStrategyOptimizer(bt *backtest.Backtester, store RecordStore, cfg OptimizerConfig) *StrategyOptimizer {
	return &StrategyOptimizer{
		backtester: bt,
		store:      store,
		config:     cfg,
	}
}

// CandidateResult pairs a parameter set with its backtest result.
type CandidateResult struct {
	Params strategy.StrategyParams
	Result *backtest.BacktestResult
}

// GenerateCandidates generates candidate parameter sets within ±MaxParamChange of current params.
func (o *StrategyOptimizer) GenerateCandidates(currentParams strategy.StrategyParams, numCandidates int) []strategy.StrategyParams {
	candidates := make([]strategy.StrategyParams, 0, numCandidates)
	maxChange := o.config.MaxParamChange

	// Generate evenly spaced candidates within the allowed range
	for i := 0; i < numCandidates; i++ {
		candidate := make(strategy.StrategyParams)
		ratio := -maxChange + (2*maxChange)*float64(i)/float64(numCandidates)

		for name, value := range currentParams {
			change := value.Mul(decimal.NewFromFloat(ratio))
			newVal := value.Add(change)
			if newVal.LessThanOrEqual(decimal.Zero) {
				newVal = value.Mul(decimal.NewFromFloat(0.1)) // floor at 10% of original
			}
			candidate[name] = newVal
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// ClampParams ensures each parameter change stays within MaxParamChange of the original.
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
			// Clamp to max allowed change
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

// SelectBest picks the candidate with the highest net profit rate.
// Returns nil if no candidate has positive net profit.
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
		return nil // no positive candidate
	}
	return best
}

// ShouldApply determines if the optimized params should replace current params.
// Returns true if the best candidate has positive net profit AND outperforms current.
func ShouldApply(currentResult, bestResult *backtest.BacktestResult) bool {
	if bestResult == nil || currentResult == nil {
		return false
	}
	// Best must have positive net profit
	if bestResult.NetProfit.IsNegative() || bestResult.NetProfit.IsZero() {
		return false
	}
	// Best must outperform current
	return bestResult.NetProfit.GreaterThan(currentResult.NetProfit)
}

// RunOptimization runs a full optimization cycle for a strategy.
func (o *StrategyOptimizer) RunOptimization(ctx context.Context, strat strategy.Strategy, btCfg backtest.BacktestConfig) (*CandidateResult, bool, error) {
	currentParams := strat.GetParams()

	// Run backtest with current params
	currentResult, err := o.backtester.Run(btCfg)
	if err != nil {
		return nil, false, fmt.Errorf("backtest with current params failed: %w", err)
	}

	// Generate candidates
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

	// Restore original params
	strat.SetParams(currentParams)

	best := SelectBest(candidateResults)
	applied := false

	if best != nil && ShouldApply(currentResult, best.Result) {
		strat.SetParams(best.Params)
		applied = true
	}

	// Save optimization record
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

// GetHistory retrieves all optimization records.
func (o *StrategyOptimizer) GetHistory() ([]model.OptimizationRecord, error) {
	if o.store == nil {
		return nil, nil
	}
	return o.store.GetAll()
}

// ParamChangeRatio calculates the change ratio for each parameter.
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
