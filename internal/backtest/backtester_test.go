package backtest

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"money-loves-me/internal/strategy"
	"money-loves-me/pkg/binance"
)

// mockKlineProvider returns pre-configured klines.
type mockKlineProvider struct {
	klines []binance.Kline
}

func (m *mockKlineProvider) GetHistoricalKlines(symbol, interval string, start, end time.Time) ([]binance.Kline, error) {
	return m.klines, nil
}

// alternatingStrategy generates alternating BUY/SELL signals for testing.
type alternatingStrategy struct {
	callCount int
	price     decimal.Decimal
	qty       decimal.Decimal
	profit    decimal.Decimal
}

func (s *alternatingStrategy) Name() string { return "ALTERNATING" }
func (s *alternatingStrategy) Calculate(klines []binance.Kline) *strategy.Signal {
	s.callCount++
	if s.callCount%2 == 1 {
		return &strategy.Signal{
			Strategy: "ALTERNATING", Side: strategy.SignalBuy,
			Price: s.price, Quantity: s.qty, ExpectedProfit: s.profit,
			Timestamp: time.Now(),
		}
	}
	return &strategy.Signal{
		Strategy: "ALTERNATING", Side: strategy.SignalSell,
		Price: s.price, Quantity: s.qty, ExpectedProfit: s.profit,
		Timestamp: time.Now(),
	}
}
func (s *alternatingStrategy) GetParams() strategy.StrategyParams {
	return strategy.StrategyParams{"period": decimal.NewFromInt(1)}
}
func (s *alternatingStrategy) SetParams(p strategy.StrategyParams) error { return nil }
func (s *alternatingStrategy) EstimateFee(price, qty, rate decimal.Decimal) decimal.Decimal {
	return price.Mul(qty).Mul(rate)
}

func generateTestKlines(n int, basePrice float64) []binance.Kline {
	klines := make([]binance.Kline, n)
	for i := 0; i < n; i++ {
		p := decimal.NewFromFloat(basePrice + float64(i)*0.1)
		klines[i] = binance.Kline{
			OpenTime:  time.Now().Add(time.Duration(-n+i) * time.Minute),
			Open:      p,
			High:      p.Add(decimal.NewFromFloat(1)),
			Low:       p.Sub(decimal.NewFromFloat(1)),
			Close:     p,
			Volume:    decimal.NewFromFloat(100),
			CloseTime: time.Now().Add(time.Duration(-n+i+1) * time.Minute),
		}
	}
	return klines
}

// Feature: binance-trading-system, Property 19: 回测手续费计算正确性
// For any backtest trade, the fee equals the trade amount times the fee rate;
// the total fees equal the sum of all individual trade fees.
// **Validates: Requirements 10.2**
func TestProperty19_BacktestFeeCalculation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random fee rate and trade parameters
		feeRateFloat := rapid.Float64Range(0.0001, 0.01).Draw(t, "feeRate")
		feeRate := decimal.NewFromFloat(feeRateFloat)
		numTrades := rapid.IntRange(1, 20).Draw(t, "numTrades")

		var trades []BacktestTrade
		expectedTotalFee := decimal.Zero

		for i := 0; i < numTrades; i++ {
			amount := decimal.NewFromFloat(rapid.Float64Range(10, 10000).Draw(t, "amount"))
			fee := CalculateFee(amount, feeRate)

			// Verify: fee == amount * feeRate
			expected := amount.Mul(feeRate)
			assert.True(t, fee.Equal(expected),
				"fee (%s) should equal amount (%s) * feeRate (%s) = %s",
				fee.String(), amount.String(), feeRate.String(), expected.String())

			trades = append(trades, BacktestTrade{Fee: fee, Amount: amount})
			expectedTotalFee = expectedTotalFee.Add(fee)
		}

		// Verify: total fees == sum of individual fees
		actualTotal := decimal.Zero
		for _, tr := range trades {
			actualTotal = actualTotal.Add(tr.Fee)
		}
		assert.True(t, actualTotal.Equal(expectedTotalFee),
			"total fees (%s) should equal sum of individual fees (%s)",
			actualTotal.String(), expectedTotalFee.String())
	})
}

// Feature: binance-trading-system, Property 20: 回测报告指标一致性
// Net profit equals total return * initial capital; win rate equals winning trades / total sell trades;
// total trades equals the length of the trades list.
// **Validates: Requirements 10.3**
func TestProperty20_BacktestReportMetricConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		price := rapid.Float64Range(100, 1000).Draw(t, "price")
		qty := rapid.Float64Range(0.1, 10).Draw(t, "qty")
		feeRate := rapid.Float64Range(0.0001, 0.005).Draw(t, "feeRate")
		numKlines := rapid.IntRange(20, 60).Draw(t, "numKlines")

		klines := generateTestKlines(numKlines, price)
		provider := &mockKlineProvider{klines: klines}
		bt := NewBacktester(provider, nil)

		strat := &alternatingStrategy{
			price:  decimal.NewFromFloat(price),
			qty:    decimal.NewFromFloat(qty),
			profit: decimal.NewFromFloat(price * qty * 0.1),
		}

		cfg := BacktestConfig{
			Symbol:     "BTCUSDT",
			Strategy:   strat,
			StartTime:  time.Now().Add(-time.Hour),
			EndTime:    time.Now(),
			InitialCap: decimal.NewFromFloat(100000),
			FeeRate:    strategy.FeeRate{Maker: decimal.NewFromFloat(feeRate), Taker: decimal.NewFromFloat(feeRate)},
			Slippage:   decimal.NewFromFloat(0.001),
		}

		result, err := bt.Run(cfg)
		assert.NoError(t, err)
		if result == nil || result.TotalTrades == 0 {
			return
		}

		// Property: TotalTrades == len(Trades)
		assert.Equal(t, result.TotalTrades, len(result.Trades),
			"TotalTrades (%d) should equal len(Trades) (%d)", result.TotalTrades, len(result.Trades))

		// Property: NetProfit == TotalReturn * InitialCap
		expectedNet := result.TotalReturn.Mul(cfg.InitialCap)
		diff := result.NetProfit.Sub(expectedNet).Abs()
		tolerance := decimal.NewFromFloat(0.01)
		assert.True(t, diff.LessThan(tolerance),
			"NetProfit (%s) should equal TotalReturn (%s) * InitialCap (%s) = %s (diff=%s)",
			result.NetProfit.String(), result.TotalReturn.String(), cfg.InitialCap.String(),
			expectedNet.String(), diff.String())

		// Property: TotalFees == sum of individual trade fees
		sumFees := decimal.Zero
		for _, tr := range result.Trades {
			sumFees = sumFees.Add(tr.Fee)
		}
		assert.True(t, result.TotalFees.Equal(sumFees),
			"TotalFees (%s) should equal sum of trade fees (%s)",
			result.TotalFees.String(), sumFees.String())

		// Property: WinRate == wins / sellCount
		sellCount := 0
		wins := 0
		for _, tr := range result.Trades {
			if tr.Side == "SELL" {
				sellCount++
				if tr.PnL.GreaterThan(decimal.Zero) {
					wins++
				}
			}
		}
		if sellCount > 0 {
			expectedWinRate := decimal.NewFromInt(int64(wins)).Div(decimal.NewFromInt(int64(sellCount)))
			assert.True(t, result.WinRate.Equal(expectedWinRate),
				"WinRate (%s) should equal %d/%d = %s",
				result.WinRate.String(), wins, sellCount, expectedWinRate.String())
		}
	})
}

// Feature: binance-trading-system, Property 21: 回测滑点模拟
// For any backtest trade, the actual execution price differs from the signal price
// by exactly the slippage percentage times the signal price.
// Buy: exec = signal + signal*slippage, Sell: exec = signal - signal*slippage
// **Validates: Requirements 10.5**
func TestProperty21_BacktestSlippageSimulation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		signalPrice := decimal.NewFromFloat(rapid.Float64Range(10, 100000).Draw(t, "signalPrice"))
		slippage := decimal.NewFromFloat(rapid.Float64Range(0, 0.05).Draw(t, "slippage"))

		// Test BUY slippage: exec price should be higher
		buyExec := ApplySlippage(signalPrice, slippage, "BUY")
		expectedBuy := signalPrice.Add(signalPrice.Mul(slippage))
		assert.True(t, buyExec.Equal(expectedBuy),
			"BUY exec price (%s) should equal signal (%s) + signal*slippage (%s) = %s",
			buyExec.String(), signalPrice.String(), slippage.String(), expectedBuy.String())

		// Test SELL slippage: exec price should be lower
		sellExec := ApplySlippage(signalPrice, slippage, "SELL")
		expectedSell := signalPrice.Sub(signalPrice.Mul(slippage))
		assert.True(t, sellExec.Equal(expectedSell),
			"SELL exec price (%s) should equal signal (%s) - signal*slippage (%s) = %s",
			sellExec.String(), signalPrice.String(), slippage.String(), expectedSell.String())

		// The difference should be exactly slippage * signalPrice
		buyDiff := buyExec.Sub(signalPrice)
		sellDiff := signalPrice.Sub(sellExec)
		expectedDiff := signalPrice.Mul(slippage)
		assert.True(t, buyDiff.Equal(expectedDiff),
			"BUY price diff (%s) should equal slippage*signal (%s)", buyDiff.String(), expectedDiff.String())
		assert.True(t, sellDiff.Equal(expectedDiff),
			"SELL price diff (%s) should equal slippage*signal (%s)", sellDiff.String(), expectedDiff.String())
	})
}

func TestBacktester_RunBasic(t *testing.T) {
	klines := generateTestKlines(30, 100)
	provider := &mockKlineProvider{klines: klines}
	bt := NewBacktester(provider, nil)

	strat := &alternatingStrategy{
		price:  decimal.NewFromFloat(100),
		qty:    decimal.NewFromFloat(1),
		profit: decimal.NewFromFloat(10),
	}

	cfg := BacktestConfig{
		Symbol:     "BTCUSDT",
		Strategy:   strat,
		StartTime:  time.Now().Add(-time.Hour),
		EndTime:    time.Now(),
		InitialCap: decimal.NewFromFloat(10000),
		FeeRate:    strategy.FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
		Slippage:   decimal.NewFromFloat(0.001),
	}

	result, err := bt.Run(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, result.TotalTrades, 0)
	assert.NotEmpty(t, result.EquityCurve)
}

func TestBacktester_NilStrategy(t *testing.T) {
	bt := NewBacktester(&mockKlineProvider{}, nil)
	_, err := bt.Run(BacktestConfig{})
	assert.Error(t, err)
}

func TestBacktester_BatchRun(t *testing.T) {
	klines := generateTestKlines(30, 100)
	provider := &mockKlineProvider{klines: klines}
	bt := NewBacktester(provider, nil)

	configs := make([]BacktestConfig, 3)
	for i := range configs {
		configs[i] = BacktestConfig{
			Symbol:     "BTCUSDT",
			Strategy:   &alternatingStrategy{price: decimal.NewFromFloat(100), qty: decimal.NewFromFloat(1), profit: decimal.NewFromFloat(10)},
			StartTime:  time.Now().Add(-time.Hour),
			EndTime:    time.Now(),
			InitialCap: decimal.NewFromFloat(10000),
			FeeRate:    strategy.FeeRate{Maker: decimal.NewFromFloat(0.001), Taker: decimal.NewFromFloat(0.001)},
			Slippage:   decimal.NewFromFloat(0.001),
		}
	}

	results, err := bt.BatchRun(configs)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
}
