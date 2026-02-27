package strategy

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"money-loves-me/pkg/binance"
)

// generateKlines 从基础价格开始创建 n 根 K 线，价格逐步递增。
func generateKlines(n int, basePrice decimal.Decimal) []binance.Kline {
	klines := make([]binance.Kline, n)
	for i := 0; i < n; i++ {
		price := basePrice.Add(decimal.NewFromInt(int64(i)))
		klines[i] = binance.Kline{
			OpenTime:  time.Now().Add(time.Duration(-n+i) * time.Minute),
			Open:      price,
			High:      price.Add(decimal.NewFromFloat(1)),
			Low:       price.Sub(decimal.NewFromFloat(1)),
			Close:     price,
			Volume:    decimal.NewFromFloat(100),
			CloseTime: time.Now().Add(time.Duration(-n+i+1) * time.Minute),
		}
	}
	return klines
}

// Feature: binance-trading-system, Property 7: 策略自动初始化有效默认参数
// 对于任何内置策略，GetParams() 应返回有效的默认参数集，
// 其中所有参数值均为正数。
// **Validates: Requirements 4.2**
func TestProperty7_StrategyDefaultParamsAreValid(t *testing.T) {
	builtInFactories := []func() Strategy{
		func() Strategy { return NewMACrossStrategy() },
		func() Strategy { return NewRSIStrategy() },
		func() Strategy { return NewBollingerStrategy() },
	}

	rapid.Check(t, func(t *rapid.T) {
		// 随机选择一个内置策略
		idx := rapid.IntRange(0, len(builtInFactories)-1).Draw(t, "strategyIndex")
		strategy := builtInFactories[idx]()

		params := strategy.GetParams()

		// 所有参数必须存在（非空映射）
		assert.NotEmpty(t, params, "strategy %s should have default parameters", strategy.Name())

		// 所有参数值必须为正数
		for name, value := range params {
			assert.True(t, value.GreaterThan(decimal.Zero),
				"strategy %s parameter %s should be positive, got %s", strategy.Name(), name, value.String())
		}

		// 验证 SetParams 接受默认参数（它们应该是有效的）
		err := strategy.SetParams(params)
		assert.NoError(t, err, "strategy %s should accept its own default params", strategy.Name())
	})
}

// --- 策略引擎核心逻辑的单元测试 ---

func TestStrategyEngine_StartStop(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	// 初始状态不应该在运行
	assert.False(t, engine.IsRunning())

	// 启动
	err := engine.Start(context.Background())
	assert.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// 重复启动应该报错
	err = engine.Start(context.Background())
	assert.Error(t, err)

	// 停止
	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.IsRunning())

	// 重复停止应该报错
	err = engine.Stop()
	assert.Error(t, err)
}

func TestStrategyEngine_EvaluateMarket_NotRunning(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	// 未运行时应返回 nil
	signals := engine.EvaluateMarket("BTCUSDT", nil)
	assert.Nil(t, signals)
}

func TestStrategyEngine_StrategyLogs(t *testing.T) {
	engine := NewStrategyEngine(
		[]Strategy{NewMACrossStrategy()},
		nil,
		FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
	)

	err := engine.Start(context.Background())
	assert.NoError(t, err)
	defer engine.Stop()

	// 使用不足的数据进行评估 - 应记录日志但无信号
	klines := generateKlines(5, decimal.NewFromFloat(100))
	signals := engine.EvaluateMarket("BTCUSDT", klines)
	assert.Empty(t, signals)

	// 检查日志是否已创建
	logs := engine.GetStrategyLogs(MACrossName)
	assert.NotEmpty(t, logs)
	assert.Equal(t, MACrossName, logs[0].StrategyName)
	assert.Equal(t, "BTCUSDT", logs[0].Symbol)
}

// Feature: binance-trading-system, Property 8: 停止交易后不产生新信号
// 调用 Stop() 后，无论收到多少市场数据更新，
// 都不应生成新的交易信号。
// **Validates: Requirements 4.6**
func TestProperty8_NoSignalsAfterStop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成停止后要发送的随机市场更新数量
		numUpdates := rapid.IntRange(1, 50).Draw(t, "numUpdates")
		numKlines := rapid.IntRange(30, 100).Draw(t, "numKlines")

		// 跟踪接收到的信号
		var signalsMu sync.Mutex
		var receivedSignals []Signal

		handler := SignalHandlerFunc(func(signal Signal) error {
			signalsMu.Lock()
			receivedSignals = append(receivedSignals, signal)
			signalsMu.Unlock()
			return nil
		})

		engine := NewStrategyEngine(
			[]Strategy{NewMACrossStrategy(), NewRSIStrategy(), NewBollingerStrategy()},
			handler,
			FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
		)

		// 启动后立即停止
		err := engine.Start(context.Background())
		assert.NoError(t, err)

		err = engine.Stop()
		assert.NoError(t, err)
		assert.False(t, engine.IsRunning())

		// 现在发送大量市场数据更新 - 不应生成任何信号
		for i := 0; i < numUpdates; i++ {
			basePrice := decimal.NewFromFloat(100).Add(decimal.NewFromInt(int64(i)))
			klines := generateKlines(numKlines, basePrice)
			signals := engine.EvaluateMarket("BTCUSDT", klines)
			assert.Empty(t, signals, "no signals should be generated after Stop()")
		}

		// 同时验证 ProcessSignals 也不产生任何输出
		klines := generateKlines(numKlines, decimal.NewFromFloat(200))
		err = engine.ProcessSignals("BTCUSDT", klines)
		assert.NoError(t, err)

		signalsMu.Lock()
		assert.Empty(t, receivedSignals, "signal handler should not receive any signals after Stop()")
		signalsMu.Unlock()
	})
}

// mockStrategy 是一个测试策略，始终生成具有可配置参数的信号。
type mockStrategy struct {
	name   string
	signal *Signal
	params StrategyParams
}

func (m *mockStrategy) Name() string { return m.name }

func (m *mockStrategy) Calculate(klines []binance.Kline) *Signal {
	if m.signal == nil {
		return nil
	}
	// 返回副本
	s := *m.signal
	return &s
}

func (m *mockStrategy) GetParams() StrategyParams { return m.params }

func (m *mockStrategy) SetParams(params StrategyParams) error {
	m.params = params
	return nil
}

func (m *mockStrategy) EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal {
	return price.Mul(quantity).Mul(feeRate)
}

// Feature: binance-trading-system, Property 9: 手续费感知的信号生成
// 对于任何交易信号，扣除手续费后的预期利润必须为正；
// 否则不应生成该信号。
// **Validates: Requirements 4.8**
func TestProperty9_FeeAwareSignalGeneration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机价格、数量、预期利润和费率
		price := decimal.NewFromFloat(rapid.Float64Range(10, 100000).Draw(t, "price"))
		quantity := decimal.NewFromFloat(rapid.Float64Range(0.001, 100).Draw(t, "quantity"))
		feeRateFloat := rapid.Float64Range(0.0001, 0.01).Draw(t, "feeRate")
		feeRate := decimal.NewFromFloat(feeRateFloat)

		// 计算此交易的手续费
		fee := price.Mul(quantity).Mul(feeRate)

		// 随机决定预期利润是否应高于手续费
		profitAboveFee := rapid.Bool().Draw(t, "profitAboveFee")

		var expectedProfit decimal.Decimal
		if profitAboveFee {
			// 预期利润 > 手续费 → 信号应通过
			margin := decimal.NewFromFloat(rapid.Float64Range(0.01, 100).Draw(t, "margin"))
			expectedProfit = fee.Add(margin)
		} else {
			// 预期利润 <= 手续费 → 信号应被拒绝
			// 生成介于 0 和手续费之间的利润（包含手续费）
			ratio := decimal.NewFromFloat(rapid.Float64Range(0, 1).Draw(t, "ratio"))
			expectedProfit = fee.Mul(ratio)
		}

		signal := &Signal{
			Strategy:       "TEST_STRATEGY",
			Symbol:         "BTCUSDT",
			Side:           SignalBuy,
			Price:          price,
			Quantity:       quantity,
			ExpectedProfit: expectedProfit,
			Timestamp:      time.Now(),
			Reason: SignalReason{
				Indicators:  map[string]float64{"test": 1.0},
				TriggerRule: "test rule",
				MarketState: "test state",
			},
		}

		mock := &mockStrategy{
			name:   "TEST_STRATEGY",
			signal: signal,
			params: StrategyParams{"test": decimal.NewFromInt(1)},
		}

		engine := NewStrategyEngine(
			[]Strategy{mock},
			nil,
			FeeRate{Maker: feeRate, Taker: feeRate},
		)

		err := engine.Start(context.Background())
		assert.NoError(t, err)
		defer engine.Stop()

		klines := generateKlines(5, price)
		signals := engine.EvaluateMarket("BTCUSDT", klines)

		if profitAboveFee {
			// 信号应被生成（预期利润 > 手续费）
			assert.Len(t, signals, 1, "signal should be generated when expected profit > fee")
			if len(signals) > 0 {
				// 验证信号的预期利润超过手续费
				actualFee := mock.EstimateFee(signals[0].Price, signals[0].Quantity, feeRate)
				assert.True(t, signals[0].ExpectedProfit.GreaterThan(actualFee),
					"expected profit (%s) should exceed fee (%s)",
					signals[0].ExpectedProfit.String(), actualFee.String())
			}
		} else {
			// 信号不应被生成（预期利润 <= 手续费）
			assert.Empty(t, signals, "signal should not be generated when expected profit <= fee")
		}
	})
}
