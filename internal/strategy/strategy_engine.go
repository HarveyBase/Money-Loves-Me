package strategy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"money-loves-me/pkg/binance"
)

// SignalHandler 是处理生成的交易信号的回调接口。
type SignalHandler interface {
	HandleSignal(signal Signal) error
}

// SignalHandlerFunc 是一个适配器，允许将普通函数用作 SignalHandler。
type SignalHandlerFunc func(signal Signal) error

func (f SignalHandlerFunc) HandleSignal(signal Signal) error {
	return f(signal)
}

// MarketDataProvider 为策略引擎抽象市场数据服务。
type MarketDataProvider interface {
	GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error)
}

// StrategyLogger 为策略引擎抽象日志记录。
type StrategyLogger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// StrategyLogEntry 记录一次策略执行事件。
type StrategyLogEntry struct {
	Timestamp    time.Time
	StrategyName string
	Symbol       string
	Message      string
	Signal       *Signal
}

// StrategyEngine 管理多个策略，评估市场状况，并生成手续费感知的交易信号。
type StrategyEngine struct {
	strategies    []Strategy
	signalHandler SignalHandler
	feeRate       FeeRate
	running       atomic.Bool

	mu         sync.RWMutex
	logs       map[string][]StrategyLogEntry // 策略名称 -> 日志条目
	cancelFunc context.CancelFunc
}

// NewStrategyEngine 使用给定的策略和费率创建新的 StrategyEngine。
func NewStrategyEngine(strategies []Strategy, handler SignalHandler, feeRate FeeRate) *StrategyEngine {
	return &StrategyEngine{
		strategies:    strategies,
		signalHandler: handler,
		feeRate:       feeRate,
		logs:          make(map[string][]StrategyLogEntry),
	}
}

// Start 将引擎设置为运行状态并开始处理市场数据。
// 它启动一个后台 goroutine，可通过 Stop() 或上下文取消来停止。
func (e *StrategyEngine) Start(ctx context.Context) error {
	if e.running.Load() {
		return fmt.Errorf("strategy engine is already running")
	}

	e.running.Store(true)

	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelFunc = cancel
	e.mu.Unlock()

	// 后台 goroutine，保持运行直到被停止。
	go func() {
		<-ctx.Done()
		e.running.Store(false)
	}()

	return nil
}

// Stop 停止策略引擎生成新信号。
// 已提交的订单将继续被跟踪。
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

// IsRunning 返回引擎当前是否正在运行。
func (e *StrategyEngine) IsRunning() bool {
	return e.running.Load()
}

// EvaluateMarket 对指定交易对的 K 线数据运行所有策略，
// 并返回通过手续费感知检查的信号。
// 只有预期利润 > 预估手续费的信号才会被返回。
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

		// 填充交易对名称
		signal.Symbol = symbol

		// 手续费感知过滤：仅发出预期利润 > 手续费的信号
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

// ProcessSignals 评估市场数据并将有利可图的信号发送给处理器。
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

// isProfitableAfterFee 检查信号的预期利润是否超过预估手续费。
// 信号要通过检查，扣除手续费后的预期利润必须为正。
func (e *StrategyEngine) isProfitableAfterFee(strat Strategy, signal *Signal) bool {
	if signal.Price.IsZero() || signal.Quantity.IsZero() {
		return false
	}

	// 使用 taker 费率作为保守估计
	fee := strat.EstimateFee(signal.Price, signal.Quantity, e.feeRate.Taker)

	// 预期利润必须超过手续费。
	expectedProfit := signal.ExpectedProfit

	// 如果没有设置明确的预期利润，则该信号不能被视为有利可图
	if expectedProfit.IsZero() || expectedProfit.IsNegative() {
		return false
	}

	return expectedProfit.GreaterThan(fee)
}

// GetStrategyLogs 返回指定策略的执行日志。
func (e *StrategyEngine) GetStrategyLogs(strategyName string) []StrategyLogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	logs := e.logs[strategyName]
	result := make([]StrategyLogEntry, len(logs))
	copy(result, logs)
	return result
}

// GetStrategies 返回引擎管理的策略列表。
func (e *StrategyEngine) GetStrategies() []Strategy {
	return e.strategies
}

// appendLog 为策略添加一条日志条目。
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
