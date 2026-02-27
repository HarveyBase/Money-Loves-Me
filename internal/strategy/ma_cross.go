package strategy

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

const (
	MACrossName             = "MA_CROSS"
	MACrossParamShortPeriod = "short_period"
	MACrossParamLongPeriod  = "long_period"
)

// MACrossStrategy 实现了移动平均线交叉策略。
// 当短期均线上穿长期均线时生成买入信号。
// 当短期均线下穿长期均线时生成卖出信号。
type MACrossStrategy struct {
	shortPeriod int
	longPeriod  int
}

// NewMACrossStrategy 使用默认参数创建新的 MA 交叉策略。
func NewMACrossStrategy() *MACrossStrategy {
	return &MACrossStrategy{
		shortPeriod: 7,
		longPeriod:  25,
	}
}

// Name 返回策略名称。
func (s *MACrossStrategy) Name() string {
	return MACrossName
}

// Calculate 分析 K 线数据并根据 MA 交叉返回买入/卖出信号。
// 如果数据不足或未检测到交叉则返回 nil。
func (s *MACrossStrategy) Calculate(klines []binance.Kline) *Signal {
	if len(klines) < s.longPeriod+1 {
		return nil
	}

	// 需要至少 longPeriod+1 根 K 线来检测交叉
	// （当前 K 线和前一根 K 线的均线值）。
	n := len(klines)

	// 计算当前和前一根 K 线的短期和长期均线。
	shortMACurr := calcSMA(klines[n-s.shortPeriod : n])
	shortMAPrev := calcSMA(klines[n-1-s.shortPeriod : n-1])
	longMACurr := calcSMA(klines[n-s.longPeriod : n])
	longMAPrev := calcSMA(klines[n-1-s.longPeriod : n-1])

	lastKline := klines[n-1]

	// 金叉：短期均线上穿长期均线 → 买入
	if shortMAPrev.LessThanOrEqual(longMAPrev) && shortMACurr.GreaterThan(longMACurr) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalBuy,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					fmt.Sprintf("MA%d", s.shortPeriod): shortMACurr.InexactFloat64(),
					fmt.Sprintf("MA%d", s.longPeriod):  longMACurr.InexactFloat64(),
				},
				TriggerRule: fmt.Sprintf("MA%d crossed above MA%d (golden cross)", s.shortPeriod, s.longPeriod),
				MarketState: "bullish crossover",
			},
		}
	}

	// 死叉：短期均线下穿长期均线 → 卖出
	if shortMAPrev.GreaterThanOrEqual(longMAPrev) && shortMACurr.LessThan(longMACurr) {
		return &Signal{
			Strategy:  s.Name(),
			Symbol:    "",
			Side:      SignalSell,
			Price:     lastKline.Close,
			Quantity:  decimal.Zero,
			Timestamp: time.Now(),
			Reason: SignalReason{
				Indicators: map[string]float64{
					fmt.Sprintf("MA%d", s.shortPeriod): shortMACurr.InexactFloat64(),
					fmt.Sprintf("MA%d", s.longPeriod):  longMACurr.InexactFloat64(),
				},
				TriggerRule: fmt.Sprintf("MA%d crossed below MA%d (death cross)", s.shortPeriod, s.longPeriod),
				MarketState: "bearish crossover",
			},
		}
	}

	return nil
}

// GetParams 返回当前策略参数。
func (s *MACrossStrategy) GetParams() StrategyParams {
	return StrategyParams{
		MACrossParamShortPeriod: decimal.NewFromInt(int64(s.shortPeriod)),
		MACrossParamLongPeriod:  decimal.NewFromInt(int64(s.longPeriod)),
	}
}

// SetParams 更新策略参数。
func (s *MACrossStrategy) SetParams(params StrategyParams) error {
	sp, ok := params[MACrossParamShortPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", MACrossParamShortPeriod)
	}
	lp, ok := params[MACrossParamLongPeriod]
	if !ok {
		return fmt.Errorf("missing parameter: %s", MACrossParamLongPeriod)
	}

	shortVal := int(sp.IntPart())
	longVal := int(lp.IntPart())

	if shortVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", MACrossParamShortPeriod, shortVal)
	}
	if longVal <= 0 {
		return fmt.Errorf("%s must be positive, got %d", MACrossParamLongPeriod, longVal)
	}
	if shortVal >= longVal {
		return fmt.Errorf("short_period (%d) must be less than long_period (%d)", shortVal, longVal)
	}

	s.shortPeriod = shortVal
	s.longPeriod = longVal
	return nil
}

// EstimateFee 根据给定的价格、数量和费率估算交易手续费。
func (s *MACrossStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// calcSMA 计算给定 K 线收盘价的简单移动平均线。
func calcSMA(klines []binance.Kline) decimal.Decimal {
	if len(klines) == 0 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, k := range klines {
		sum = sum.Add(k.Close)
	}
	return sum.Div(decimal.NewFromInt(int64(len(klines))))
}
