package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	RSIName            = "RSI"
	RSIParamPeriod     = "period"
	RSIParamOverbought = "overbought"
	RSIParamOversold   = "oversold"
)

// RSIStrategy 实现了 RSI 超买/超卖策略。
// 当 RSI 低于超卖水平时生成买入信号。
// 当 RSI 高于超买水平时生成卖出信号。
type RSIStrategy struct {
	period     int
	overbought decimal.Decimal
	oversold   decimal.Decimal
}

// NewRSIStrategy 使用默认参数创建新的 RSI 策略。
func NewRSIStrategy() *RSIStrategy {
	return &RSIStrategy{
		period:     14,
		overbought: decimal.NewFromInt(70),
		oversold:   decimal.NewFromInt(30),
	}
}

// Name 返回策略名称。
func (s *RSIStrategy) Name() string {
	return RSIName
}

// Calculate 分析 K 线数据并根据 RSI 水平返回买入/卖出信号。
// 如果数据不足或 RSI 处于中性区域则返回 nil。
func (s *RSIStrategy) Calculate(klines []binance.Kline) *Signal {
	// 需要至少 period+1 根 K 线来计算 RSI（period 个价格变动）。
	if len(klines) < s.period+1 {
		return nil
	}

	rsi := s.calcRSI(klines)
	if rsi.IsNegative() {
		return nil // 数据不足或除法问题
	}

	lastKline := klines[len(klines)-1]
	rsiFloat := rsi.InexactFloat64()

	// 超卖 → 买入信号
	if rsi.LessThanOrEqual(s.oversold) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					"RSI": rsiFloat,
				},
				TriggerRule: fmt.Sprintf("RSI (%.2f) <= oversold level (%.2f)", rsiFloat, s.oversold.InexactFloat64()),
				MarketState: "oversold",
			},
		}
	}

	// 超买 → 卖出信号
	if rsi.GreaterThanOrEqual(s.overbought) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					"RSI": rsiFloat,
				},
				TriggerRule: fmt.Sprintf("RSI (%.2f) >= overbought level (%.2f)", rsiFloat, s.overbought.InexactFloat64()),
				MarketState: "overbought",
			},
		}
	}

	return nil
}

// GetParams 返回当前策略参数。
func (s *RSIStrategy) GetParams() StrategyParams {
	return StrategyParams{
		RSIParamPeriod:     decimal.NewFromInt(int64(s.period)),
		RSIParamOverbought: s.overbought,
		RSIParamOversold:   s.oversold,
	}
}

// SetParams 更新策略参数。
func (s *RSIStrategy) SetParams(params StrategyParams) error {
	p, ok := params[RSIParamPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamPeriod)
	}
	ob, ok := params[RSIParamOverbought]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamOverbought)
	}
	os, ok := params[RSIParamOversold]
	if !ok {
		return fmt.Errorf("missing parameter: %s", RSIParamOversold)
	}

	periodVal := int(p.IntPart())
	if periodVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", RSIParamPeriod, periodVal)
	}
	if ob.LessThanOrEqual(decimal.Zero) || ob.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("%s must be between 0 and 100, got %s", RSIParamOverbought, ob.String())
	}
	if os.LessThan(decimal.Zero) || os.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return fmt.Errorf("%s must be between 0 and 100, got %s", RSIParamOversold, os.String())
	}
	if os.GreaterThanOrEqual(ob) {
		return fmt.Errorf("oversold (%s) must be less than overbought (%s)", os.String(), ob.String())
	}

	s.period = periodVal
	s.overbought = ob
	s.oversold = os
	return nil
}

// EstimateFee 根据给定的价格、数量和费率估算交易手续费。
func (s *RSIStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcRSI 使用最近 period+1 根 K 线计算 RSI 值。
// 使用标准 RSI 公式，基于涨幅和跌幅的简单平均。
func (s *RSIStrategy) calcRSI(klines []binance.Kline) decimal.Decimal {
	n := len(klines)
	if n < s.period+1 {
		return decimal.NewFromInt(-1)
	}

	// 使用最近的 period+1 根 K 线。
	recent := klines[n-s.period-1:]

	gains := decimal.Zero
	losses := decimal.Zero

	for i := 1; i < len(recent); i++ {
		change := recent[i].Close.Sub(recent[i-1].Close)
		if change.IsPositive() {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Abs())
		}
	}

	periodDec := decimal.NewFromInt(int64(s.period))
	avgGain := gains.Div(periodDec)
	avgLoss := losses.Div(periodDec)

	if avgLoss.IsZero() {
		return decimal.NewFromInt(100) // 无跌幅 → RSI = 100
	}
	if avgGain.IsZero() {
		return decimal.Zero // 无涨幅 → RSI = 0
	}

	rs := avgGain.Div(avgLoss)
	rsi := decimal.NewFromInt(100).Sub(
		decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)),
	)
	return rsi
}
