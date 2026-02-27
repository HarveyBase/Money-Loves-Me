package strategy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"money-loves-me/pkg/binance"
)

// SignalHandler is a callback interface for handling generated trading signals.
type SignalHandler interface {
	HandleSignal(signal Signal) error
}

// SignalHandlerFunc is an adapter to allow the use of ordinary functions as SignalHandler.
type SignalHandlerFunc func(signal Signal) error

func (f SignalHandlerFunc) HandleSignal(signal Signal) error {
	return f(signal)
}

// MarketDataProvider abstracts the market data service for the strategy engine.
type MarketDataProvider interface {
	GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error)
}

// StrategyLogger abstracts logging for the strategy engine.
type StrategyLogger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// StrategyLogEntry records a strategy execution event.
type StrategyLogEntry struct {
	Timestamp    time.Time
	StrategyName string
	Symbol       string
	Message      string
	Signal       *Signal
}

// StrategyEngine manages multiple strategies, evaluates market conditions,
// and generates fee-aware trading signals.
type StrategyEngine struct {
	strategies    []Strategy
	signalHandler SignalHandler
	feeRate       FeeRate
	running       atomic.Bool

	mu         sync.RWMutex
	logs       map[string][]StrategyLogEntry // strategyName -> log entries
	cancelFunc context.CancelFunc
}

// NewStrategyEngine creates a new StrategyEngine with the given strategies and fee rate.
func NewStrategyEngine(strategies []Strategy, handler SignalHandler, feeRate FeeRate) *StrategyEngine {
	return &StrategyEngine{
		strategies:    strategies,
		signalHandler: handler,
		feeRate:       feeRate,
		logs:          make(map[string][]StrategyLogEntry),
	}
}

// Start sets the engine to running state and begins processing market data.
// It launches a background goroutine that can be stopped via Stop() or context cancellation.
func (e *StrategyEngine) Start(ctx context.Context) error {
	if e.running.Load() {
		return fmt.Errorf("strategy engine is already running")
	}

	e.running.Store(true)

	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelFunc = cancel
	e.mu.Unlock()

	// Background goroutine that stays alive until stopped.
	go func() {
		<-ctx.Done()
		e.running.Store(false)
	}()

	return nil
}

// Stop stops the strategy engine from generating new signals.
// Already submitted orders continue to be tracked.
func (e *StrategyEngine) Stop() error {
	if !e.running.Load() {
		return fmt.Errorf("strategy engine is not running")
	}

	e.running.Store(false)

	e.mu.RLock()
	cancel := e.cancelFunc
	e.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	return nil
}

// IsRunning returns whether the engine is currently running.
func (e *StrategyEngine) IsRunning() bool {
	return e.running.Load()
}

// EvaluateMarket runs all strategies against the provided klines for a symbol
// and returns signals that pass the fee-awareness check.
// Only signals where expected profit > estimated fee are returned.
func (e *StrategyEngine) EvaluateMarket(symbol string, klines []binance.Kline) []Signal {
	if !e.running.Load() {
		return nil
	}

	var signals []Signal

	for _, strat := range e.strategies {
		signal := strat.Calculate(klines)

		e.appendLog(strat.Name(), symbol, "evaluated market data", signal)

		if signal == nil {
			continue
		}

		// Fill in the symbol
		signal.Symbol = symbol

		// Fee-aware filtering: only emit signals where expected profit > fee
		if !e.isProfitableAfterFee(strat, signal) {
			e.appendLog(strat.Name(), symbol,
				fmt.Sprintf("signal rejected: expected profit does not exceed fee (price=%s, qty=%s)",
					signal.Price.String(), signal.Quantity.String()), signal)
			continue
		}

		e.appendLog(strat.Name(), symbol,
			fmt.Sprintf("signal generated: %s %s @ %s", signal.Side, symbol, signal.Price.String()), signal)

		signals = append(signals, *signal)
	}

	return signals
}

// ProcessSignals evaluates market data and sends profitable signals to the handler.
func (e *StrategyEngine) ProcessSignals(symbol string, klines []binance.Kline) error {
	signals := e.EvaluateMarket(symbol, klines)

	for _, sig := range signals {
		if e.signalHandler != nil {
			if err := e.signalHandler.HandleSignal(sig); err != nil {
				e.appendLog(sig.Strategy, symbol,
					fmt.Sprintf("signal handler error: %v", err), &sig)
				return err
			}
		}
	}

	return nil
}

// isProfitableAfterFee checks if a signal's expected profit exceeds the estimated fee.
// For a signal to pass, the expected profit margin must be positive after fees.
func (e *StrategyEngine) isProfitableAfterFee(strat Strategy, signal *Signal) bool {
	if signal.Price.IsZero() || signal.Quantity.IsZero() {
		return false
	}

	// Use taker fee rate as the conservative estimate
	fee := strat.EstimateFee(signal.Price, signal.Quantity, e.feeRate.Taker)

	// The expected profit must exceed the fee.
	expectedProfit := signal.ExpectedProfit

	// If no explicit expected profit is set, the signal cannot be considered profitable
	if expectedProfit.IsZero() || expectedProfit.IsNegative() {
		return false
	}

	return expectedProfit.GreaterThan(fee)
}

// GetStrategyLogs returns the execution logs for a specific strategy.
func (e *StrategyEngine) GetStrategyLogs(strategyName string) []StrategyLogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	logs := e.logs[strategyName]
	result := make([]StrategyLogEntry, len(logs))
	copy(result, logs)
	return result
}

// GetStrategies returns the list of strategies managed by the engine.
func (e *StrategyEngine) GetStrategies() []Strategy {
	return e.strategies
}

// appendLog adds a log entry for a strategy.
func (e *StrategyEngine) appendLog(strategyName, symbol, message string, signal *Signal) {
	entry := StrategyLogEntry{
		Timestamp:    time.Now(),
		StrategyName: strategyName,
		Symbol:       symbol,
		Message:      message,
		Signal:       signal,
	}

	e.mu.Lock()
	e.logs[strategyName] = append(e.logs[strategyName], entry)
	e.mu.Unlock()
}
