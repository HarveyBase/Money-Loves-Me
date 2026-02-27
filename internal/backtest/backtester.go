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

// BacktestConfig 保存单次回测运行的配置。
type BacktestConfig struct {
	Symbol     string
	Strategy   strategy.Strategy
	StartTime  time.Time
	EndTime    time.Time
	InitialCap decimal.Decimal
	FeeRate    strategy.FeeRate
	Slippage   decimal.Decimal // 滑点百分比，例如 0.001 = 0.1%
}

// BacktestTrade 表示回测期间执行的单笔交易。
type BacktestTrade struct {
	Timestamp   time.Time       `json:"timestamp"`
	Side        string          `json:"side"` // BUY 或 SELL
	SignalPrice decimal.Decimal `json:"signal_price"`
	ExecPrice   decimal.Decimal `json:"exec_price"`
	Quantity    decimal.Decimal `json:"quantity"`
	Amount      decimal.Decimal `json:"amount"` // 执行价格 * 数量
	Fee         decimal.Decimal `json:"fee"`
	PnL         decimal.Decimal `json:"pnl"` // 卖出交易的已实现盈亏
}

// EquityPoint 表示权益曲线上的一个点。
type EquityPoint struct {
	Timestamp time.Time       `json:"timestamp"`
	Equity    decimal.Decimal `json:"equity"`
}

// BacktestResult 保存回测运行的完整结果。
type BacktestResult struct {
	TotalReturn  decimal.Decimal // 总收益率
	NetProfit    decimal.Decimal // 扣除手续费后的净利润
	GrossProfit  decimal.Decimal // 扣除手续费前的毛利润
	MaxDrawdown  decimal.Decimal // 最大回撤百分比
	WinRate      decimal.Decimal // 盈利交易比率
	ProfitFactor decimal.Decimal // 盈亏比（毛利润 / 毛亏损）
	TotalTrades  int
	TotalFees    decimal.Decimal
	Trades       []BacktestTrade
	EquityCurve  []EquityPoint
}

// KlineProvider 抽象历史 K 线数据的来源。
type KlineProvider interface {
	GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error)
}

// ResultStore 抽象回测结果的持久化存储。
type ResultStore interface {
	Create(result *model.BacktestResult) error
	GetByStrategy(strategyName string) ([]model.BacktestResult, error)
}

// Backtester 对历史数据运行策略回测。
type Backtester struct {
	klineProvider KlineProvider
	store         ResultStore
}

// NewBacktester 创建新的 Backtester。
func NewBacktester(kp KlineProvider, store ResultStore) *Backtester {
	return &Backtester{klineProvider: kp, store: store}
}

// ApplySlippage 根据滑点百分比调整信号价格。
// 买入订单获得更高的价格，卖出订单获得更低的价格。
func ApplySlippage(signalPrice, slippage decimal.Decimal, side string) decimal.Decimal {
	adj := signalPrice.Mul(slippage)
	if side == "BUY" {
		return signalPrice.Add(adj)
	}
	return signalPrice.Sub(adj)
}

// CalculateFee 根据交易金额和费率计算手续费。
func CalculateFee(amount, feeRate decimal.Decimal) decimal.Decimal {
	return amount.Mul(feeRate)
}

// Run 使用给定配置执行回测。
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

// simulate 对 K 线数据运行策略并生成 BacktestResult。
func (b *Backtester) simulate(cfg BacktestConfig, klines []binance.Kline) *BacktestResult {
	equity := cfg.InitialCap
	peakEquity := equity
	maxDrawdown := decimal.Zero
	var trades []BacktestTrade
	var equityCurve []EquityPoint
	var position *openPosition // 当前持仓

	equityCurve = append(equityCurve, EquityPoint{Timestamp: klines[0].OpenTime, Equity: equity})

	// 在 K 线上滑动窗口，在每一步评估策略
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
			// 默认：使用权益的 10%
			qty = equity.Mul(decimal.NewFromFloat(0.1)).Div(execPrice)
		}

		amount := execPrice.Mul(qty)
		fee := CalculateFee(amount, cfg.FeeRate.Taker)

		if side == "BUY" && position == nil {
			// 开仓
			if equity.LessThan(amount.Add(fee)) {
				continue // 资金不足
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
			// 平仓
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

		// 更新权益曲线和回撤
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
				grossProfit = grossProfit.Add(t.PnL.Add(t.Fee).Add(trades[sellCount*2-2].Fee)) // 近似毛利润
			} else {
				grossLoss = grossLoss.Add(t.PnL.Abs())
			}
		}
	}

	netProfit := finalEquity.Sub(cfg.InitialCap)
	grossProfitTotal := netProfit.Add(totalFees) // 毛利润 = 净利润 + 手续费

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

// BatchRun 运行多个回测配置并返回所有结果。
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

// SaveResult 将回测结果持久化到存储中。
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

// GetResults 获取指定策略的历史回测结果。
func (b *Backtester) GetResults(strategyName string) ([]model.BacktestResult, error) {
	if b.store == nil {
		return nil, nil
	}
	return b.store.GetByStrategy(strategyName)
}
