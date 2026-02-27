package backtest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/internal/model"
	"money-loves-me/internal/strategy"
	"money-loves-me/pkg/binance"
)

// BacktestConfig holds the configuration for a single backtest run.
type BacktestConfig struct {
	Symbol     string
	Strategy   strategy.Strategy
	StartTime  time.Time
	EndTime    time.Time
	InitialCap decimal.Decimal
	FeeRate    strategy.FeeRate
	Slippage   decimal.Decimal // slippage percentage, e.g. 0.001 = 0.1%
}

// BacktestTrade represents a single trade executed during backtesting.
type BacktestTrade struct {
	Timestamp   time.Time       `json:"timestamp"`
	Side        string          `json:"side"` // BUY or SELL
	SignalPrice decimal.Decimal `json:"signal_price"`
	ExecPrice   decimal.Decimal `json:"exec_price"`
	Quantity    decimal.Decimal `json:"quantity"`
	Amount      decimal.Decimal `json:"amount"` // exec_price * quantity
	Fee         decimal.Decimal `json:"fee"`
	PnL         decimal.Decimal `json:"pnl"` // realized PnL for SELL trades
}

// EquityPoint represents a point on the equity curve.
type EquityPoint struct {
	Timestamp time.Time       `json:"timestamp"`
	Equity    decimal.Decimal `json:"equity"`
}

// BacktestResult holds the complete results of a backtest run.
type BacktestResult struct {
	TotalReturn  decimal.Decimal // total return percentage
	NetProfit    decimal.Decimal // net profit after fees
	GrossProfit  decimal.Decimal // gross profit before fees
	MaxDrawdown  decimal.Decimal // maximum drawdown percentage
	WinRate      decimal.Decimal // winning trade ratio
	ProfitFactor decimal.Decimal // profit factor (gross profit / gross loss)
	TotalTrades  int
	TotalFees    decimal.Decimal
	Trades       []BacktestTrade
	EquityCurve  []EquityPoint
}

// KlineProvider abstracts the source of historical kline data.
type KlineProvider interface {
	GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error)
}

// ResultStore abstracts persistence of backtest results.
type ResultStore interface {
	Create(result *model.BacktestResult) error
	GetByStrategy(strategyName string) ([]model.BacktestResult, error)
}

// Backtester runs strategy backtests against historical data.
type Backtester struct {
	klineProvider KlineProvider
	store         ResultStore
}

// NewBacktester creates a new Backtester.
func NewBacktester(kp KlineProvider, store ResultStore) *Backtester {
	return &Backtester{klineProvider: kp, store: store}
}

// ApplySlippage adjusts a signal price by the slippage percentage.
// Buy orders get a higher price, sell orders get a lower price.
func ApplySlippage(signalPrice, slippage decimal.Decimal, side string) decimal.Decimal {
	adj := signalPrice.Mul(slippage)
	if side == "BUY" {
		return signalPrice.Add(adj)
	}
	return signalPrice.Sub(adj)
}

// CalculateFee computes the fee for a trade given amount and fee rate.
func CalculateFee(amount, feeRate decimal.Decimal) decimal.Decimal {
	return amount.Mul(feeRate)
}

// Run executes a backtest with the given configuration.
func (b *Backtester) Run(cfg BacktestConfig) (*BacktestResult, error) {
	if cfg.Strategy == nil {
		return nil, fmt.Errorf("strategy is required")
	}
	if cfg.InitialCap.IsZero() || cfg.InitialCap.IsNegative() {
		return nil, fmt.Errorf("initial capital must be positive")
	}

	klines, err := b.klineProvider.GetHistoricalKlines(cfg.Symbol, "1m", cfg.StartTime, cfg.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}
	if len(klines) == 0 {
		return &BacktestResult{}, nil
	}

	return b.simulate(cfg, klines), nil
}

// simulate runs the strategy against klines and produces a BacktestResult.
func (b *Backtester) simulate(cfg BacktestConfig, klines []binance.Kline) *BacktestResult {
	equity := cfg.InitialCap
	peakEquity := equity
	maxDrawdown := decimal.Zero
	var trades []BacktestTrade
	var equityCurve []EquityPoint
	var position *openPosition // current open position

	equityCurve = append(equityCurve, EquityPoint{Timestamp: klines[0].OpenTime, Equity: equity})

	// Slide a window over klines and evaluate strategy at each step
	for i := 1; i < len(klines); i++ {
		window := klines[:i+1]
		signal := cfg.Strategy.Calculate(window)
		if signal == nil {
			continue
		}

		side := string(signal.Side)
		signalPrice := signal.Price
		if signalPrice.IsZero() {
			signalPrice = klines[i].Close
		}
		execPrice := ApplySlippage(signalPrice, cfg.Slippage, side)
		qty := signal.Quantity
		if qty.IsZero() {
			// Default: use 10% of equity
			qty = equity.Mul(decimal.NewFromFloat(0.1)).Div(execPrice)
		}

		amount := execPrice.Mul(qty)
		fee := CalculateFee(amount, cfg.FeeRate.Taker)

		if side == "BUY" && position == nil {
			// Open position
			if equity.LessThan(amount.Add(fee)) {
				continue // not enough capital
			}
			equity = equity.Sub(amount).Sub(fee)
			position = &openPosition{
				entryPrice: execPrice,
				quantity:   qty,
				entryFee:   fee,
			}
			trades = append(trades, BacktestTrade{
				Timestamp:   klines[i].CloseTime,
				Side:        "BUY",
				SignalPrice: signalPrice,
				ExecPrice:   execPrice,
				Quantity:    qty,
				Amount:      amount,
				Fee:         fee,
			})
		} else if side == "SELL" && position != nil {
			// Close position
			sellAmount := execPrice.Mul(position.quantity)
			sellFee := CalculateFee(sellAmount, cfg.FeeRate.Taker)
			buyAmount := position.entryPrice.Mul(position.quantity)
			pnl := sellAmount.Sub(buyAmount).Sub(position.entryFee).Sub(sellFee)

			equity = equity.Add(sellAmount).Sub(sellFee)
			trades = append(trades, BacktestTrade{
				Timestamp:   klines[i].CloseTime,
				Side:        "SELL",
				SignalPrice: signalPrice,
				ExecPrice:   execPrice,
				Quantity:    position.quantity,
				Amount:      sellAmount,
				Fee:         sellFee,
				PnL:         pnl,
			})
			position = nil
		}

		// Update equity curve and drawdown
		if equity.GreaterThan(peakEquity) {
			peakEquity = equity
		}
		if peakEquity.GreaterThan(decimal.Zero) {
			dd := peakEquity.Sub(equity).Div(peakEquity)
			if dd.GreaterThan(maxDrawdown) {
				maxDrawdown = dd
			}
		}
		equityCurve = append(equityCurve, EquityPoint{Timestamp: klines[i].CloseTime, Equity: equity})
	}

	return b.buildResult(cfg, trades, equityCurve, equity, maxDrawdown)
}

type openPosition struct {
	entryPrice decimal.Decimal
	quantity   decimal.Decimal
	entryFee   decimal.Decimal
}

func (b *Backtester) buildResult(cfg BacktestConfig, trades []BacktestTrade, curve []EquityPoint, finalEquity, maxDD decimal.Decimal) *BacktestResult {
	totalFees := decimal.Zero
	grossProfit := decimal.Zero
	grossLoss := decimal.Zero
	wins := 0
	sellCount := 0

	for _, t := range trades {
		totalFees = totalFees.Add(t.Fee)
		if t.Side == "SELL" {
			sellCount++
			if t.PnL.GreaterThan(decimal.Zero) {
				wins++
				grossProfit = grossProfit.Add(t.PnL.Add(t.Fee).Add(trades[sellCount*2-2].Fee)) // approximate gross
			} else {
				grossLoss = grossLoss.Add(t.PnL.Abs())
			}
		}
	}

	netProfit := finalEquity.Sub(cfg.InitialCap)
	grossProfitTotal := netProfit.Add(totalFees) // gross = net + fees

	totalReturn := decimal.Zero
	if cfg.InitialCap.GreaterThan(decimal.Zero) {
		totalReturn = netProfit.Div(cfg.InitialCap)
	}

	winRate := decimal.Zero
	if sellCount > 0 {
		winRate = decimal.NewFromInt(int64(wins)).Div(decimal.NewFromInt(int64(sellCount)))
	}

	profitFactor := decimal.Zero
	if grossLoss.GreaterThan(decimal.Zero) {
		profitFactor = grossProfit.Div(grossLoss)
	}

	return &BacktestResult{
		TotalReturn:  totalReturn,
		NetProfit:    netProfit,
		GrossProfit:  grossProfitTotal,
		MaxDrawdown:  maxDD,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		TotalTrades:  len(trades),
		TotalFees:    totalFees,
		Trades:       trades,
		EquityCurve:  curve,
	}
}

// BatchRun runs multiple backtest configurations and returns all results.
func (b *Backtester) BatchRun(configs []BacktestConfig) ([]*BacktestResult, error) {
	results := make([]*BacktestResult, 0, len(configs))
	for _, cfg := range configs {
		result, err := b.Run(cfg)
		if err != nil {
			return nil, fmt.Errorf("backtest failed for %s: %w", cfg.Strategy.Name(), err)
		}
		results = append(results, result)
	}
	return results, nil
}

// SaveResult persists a backtest result to the store.
func (b *Backtester) SaveResult(cfg BacktestConfig, result *BacktestResult) error {
	if b.store == nil {
		return nil
	}

	paramsJSON, _ := json.Marshal(cfg.Strategy.GetParams())
	tradesJSON, _ := json.Marshal(result.Trades)
	curveJSON, _ := json.Marshal(result.EquityCurve)

	record := &model.BacktestResult{
		StrategyName:   cfg.Strategy.Name(),
		Symbol:         cfg.Symbol,
		Params:         paramsJSON,
		StartTime:      cfg.StartTime,
		EndTime:        cfg.EndTime,
		InitialCapital: cfg.InitialCap,
		TotalReturn:    result.TotalReturn,
		NetProfit:      result.NetProfit,
		MaxDrawdown:    result.MaxDrawdown,
		WinRate:        result.WinRate,
		ProfitFactor:   result.ProfitFactor,
		TotalTrades:    result.TotalTrades,
		TotalFees:      result.TotalFees,
		EquityCurve:    curveJSON,
		Trades:         tradesJSON,
		Slippage:       cfg.Slippage,
	}
	return b.store.Create(record)
}

// GetResults retrieves historical backtest results for a strategy.
func (b *Backtester) GetResults(strategyName string) ([]model.BacktestResult, error) {
	if b.store == nil {
		return nil, nil
	}
	return b.store.GetByStrategy(strategyName)
}
