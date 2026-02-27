package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	BollingerName            = "BOLLINGER"
	BollingerParamPeriod     = "period"
	BollingerParamStdDevMult = "std_dev_multiplier"
)

// BollingerStrategy 实现了布林带突破策略。
// 当价格触及或跌破下轨时生成买入信号。
// 当价格触及或突破上轨时生成卖出信号。
type BollingerStrategy struct {
	period     int
	stdDevMult decimal.Decimal
}

// NewBollingerStrategy 使用默认参数创建新的布林带策略。
func NewBollingerStrategy() *BollingerStrategy {
	return &BollingerStrategy{
		period:     20,
		stdDevMult: decimal.NewFromFloat(2.0),
	}
}

// Name 返回策略名称。
func (s *BollingerStrategy) Name() string {
	return BollingerName
}

// Calculate 分析 K 线数据并根据布林带返回买入/卖出信号。
// 如果数据不足或价格在布林带内则返回 nil。
func (s *BollingerStrategy) Calculate(klines []binance.Kline) *Signal {
	if len(klines) < s.period {
		return nil
	}

	n := len(klines)
	recent := klines[n-s.period:]

	middle := calcSMA(recent)
	stdDev := s.calcStdDev(recent, middle)

	upper := middle.Add(s.stdDevMult.Mul(stdDev))
	lower := middle.Sub(s.stdDevMult.Mul(stdDev))

	lastKline := klines[n-1]
	price := lastKline.Close

	indicators := map[string]float64{
		"BB_Upper":  upper.InexactFloat64(),
		"BB_Middle": middle.InexactFloat64(),
		"BB_Lower":  lower.InexactFloat64(),
		"BB_StdDev": stdDev.InexactFloat64(),
	}

	// 价格触及或低于下轨 → 买入
	if price.LessThanOrEqual(lower) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     price,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators:  indicators,
				TriggerRule: fmt.Sprintf("Price (%.2f) <= lower band (%.2f)", price.InexactFloat64(), lower.InexactFloat64()),
				MarketState: "oversold - price at lower Bollinger Band",
			},
		}
	}

	// 价格触及或高于上轨 → 卖出
	if price.GreaterThanOrEqual(upper) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     price,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators:  indicators,
				TriggerRule: fmt.Sprintf("Price (%.2f) >= upper band (%.2f)", price.InexactFloat64(), upper.InexactFloat64()),
				MarketState: "overbought - price at upper Bollinger Band",
			},
		}
	}

	return nil
}

// GetParams 返回当前策略参数。
func (s *BollingerStrategy) GetParams() StrategyParams {
	return StrategyParams{
		BollingerParamPeriod:     decimal.NewFromInt(int64(s.period)),
		BollingerParamStdDevMult: s.stdDevMult,
	}
}

// SetParams 更新策略参数。
func (s *BollingerStrategy) SetParams(params StrategyParams) error {
	p, ok := params[BollingerParamPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", BollingerParamPeriod)
	}
	m, ok := params[BollingerParamStdDevMult]
	if !ok {
		return fmt.Errorf("missing parameter: %s", BollingerParamStdDevMult)
	}

	periodVal := int(p.IntPart())
	if periodVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", BollingerParamPeriod, periodVal)
	}
	if m.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%s must be positive, got %s", BollingerParamStdDevMult, m.String())
	}

	s.period = periodVal
	s.stdDevMult = m
	return nil
}

// EstimateFee 根据给定的价格、数量和费率估算交易手续费。
func (s *BollingerStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcStdDev 计算收盘价的总体标准差。
func (s *BollingerStrategy) calcStdDev(klines []binance.Kline, mean decimal.Decimal) decimal.Decimal {
	if len(klines) == 0 {
		return decimal.Zero
	}

	sumSquares := decimal.Zero
	for _, k := range klines {
		diff := k.Close.Sub(mean)
		sumSquares = sumSquares.Add(diff.Mul(diff))
	}

	variance := sumSquares.Div(decimal.NewFromInt(int64(len(klines))))
	return decimalSqrt(variance)
}

// decimalSqrt 使用牛顿法计算 decimal 的平方根。
func decimalSqrt(d decimal.Decimal) decimal.Decimal {
	if d.IsZero() || d.IsNegative() {
		return decimal.Zero
	}

	// 牛顿法：x_{n+1} = (x_n + d/x_n) / 2
	two := decimal.NewFromInt(2)
	x := d // 初始猜测值
	for i := 0; i < 50; i++ {
		next := x.Add(d.Div(x)).Div(two)
		// 以高精度检查收敛性。
		if next.Sub(x).Abs().LessThan(decimal.New(1, -16)) {
			return next
		}
		x = next
	}
	return x
}
