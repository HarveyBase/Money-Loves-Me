package risk

import (
	"encoding/json"
	"money-loves-me/internal/model"
	"money-loves-me/internal/notification"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// --- 测试替身 ---

type mockRiskStore struct {
	config *model.RiskConfig
	err    error
}

func (m *mockRiskStore) Get() (*model.RiskConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.config, nil
}

func (m *mockRiskStore) Save(config *model.RiskConfig) error {
	if m.err != nil {
		return m.err
	}
	m.config = config
	return nil
}

type mockPauser struct {
	called bool
	err    error
}

func (m *mockPauser) PauseAll() error {
	m.called = true
	return m.err
}

type mockNotifStore struct {
	notifications []*model.Notification
}

func (m *mockNotifStore) Create(n *model.Notification) error {
	m.notifications = append(m.notifications, n)
	return nil
}

func (m *mockNotifStore) GetByFilter(_ notification.NotificationFilter) ([]model.Notification, error) {
	return nil, nil
}

func (m *mockNotifStore) MarkAsRead(_ int64) error {
	return nil
}

func newTestRiskManager(store RiskStore, pauser StrategyPauser) (*RiskManager, *mockNotifStore) {
	notifStore := &mockNotifStore{}
	notifier := notification.NewNotificationService(notifStore, nil)
	rm := NewRiskManager(store, notifier, pauser, nil)
	return rm, notifStore
}

// --- CheckOrder 测试 ---

func TestCheckOrder_WithinLimits(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		MaxOrderAmount:   decimal.NewFromInt(1000),
		MaxPositionRatio: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(0.5)},
	})

	err := rm.CheckOrder("BTCUSDT", decimal.NewFromInt(500), decimal.NewFromInt(10000))
	assert.NoError(t, err)
}

func TestCheckOrder_ExceedsMaxAmount(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		MaxOrderAmount: decimal.NewFromInt(1000),
	})

	err := rm.CheckOrder("BTCUSDT", decimal.NewFromInt(1500), decimal.NewFromInt(10000))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max order amount")
}

func TestCheckOrder_ExceedsPositionRatio(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		MaxOrderAmount:   decimal.NewFromInt(10000),
		MaxPositionRatio: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(0.1)},
	})

	// 2000 / 10000 = 0.2 > 0.1
	err := rm.CheckOrder("BTCUSDT", decimal.NewFromInt(2000), decimal.NewFromInt(10000))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max position ratio")
}

func TestCheckOrder_NoLimitsConfigured(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	// 零值配置表示没有限制。
	err := rm.CheckOrder("BTCUSDT", decimal.NewFromInt(999999), decimal.NewFromInt(10000))
	assert.NoError(t, err)
}

// --- CheckDailyLoss 测试 ---

func TestCheckDailyLoss_BelowThreshold(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{MaxDailyLoss: decimal.NewFromInt(1000)})

	now := time.Now()
	trades := []model.Trade{
		{Side: "BUY", Amount: decimal.NewFromInt(500), Fee: decimal.NewFromInt(1), ExecutedAt: now},
		{Side: "SELL", Amount: decimal.NewFromInt(400), Fee: decimal.NewFromInt(1), ExecutedAt: now},
	}
	// 亏损 = 500 - 400 + 1 + 1 = 102
	shouldPause, loss := rm.CheckDailyLoss(trades)
	assert.False(t, shouldPause)
	assert.True(t, loss.Equal(decimal.NewFromInt(102)))
}

func TestCheckDailyLoss_ExceedsThreshold(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{MaxDailyLoss: decimal.NewFromInt(100)})

	now := time.Now()
	trades := []model.Trade{
		{Side: "BUY", Amount: decimal.NewFromInt(500), Fee: decimal.NewFromInt(5), ExecutedAt: now},
		{Side: "SELL", Amount: decimal.NewFromInt(300), Fee: decimal.NewFromInt(5), ExecutedAt: now},
	}
	// 亏损 = 500 - 300 + 5 + 5 = 210
	shouldPause, loss := rm.CheckDailyLoss(trades)
	assert.True(t, shouldPause)
	assert.True(t, loss.Equal(decimal.NewFromInt(210)))
}

func TestCheckDailyLoss_IgnoresYesterdayTrades(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{MaxDailyLoss: decimal.NewFromInt(100)})

	yesterday := time.Now().Add(-24 * time.Hour)
	trades := []model.Trade{
		{Side: "BUY", Amount: decimal.NewFromInt(5000), Fee: decimal.NewFromInt(50), ExecutedAt: yesterday},
	}
	shouldPause, loss := rm.CheckDailyLoss(trades)
	assert.False(t, shouldPause)
	assert.True(t, loss.IsZero())
}

// --- GenerateStopLossSignal 测试 ---

func TestGenerateStopLossSignal_TriggersAtThreshold(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		StopLossPercent: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromInt(5)},
	})

	// 入场价 100，当前价 94 → 亏损 = 6%
	signal := rm.GenerateStopLossSignal("BTCUSDT",
		decimal.NewFromInt(100), decimal.NewFromInt(94), decimal.NewFromInt(1))
	require.NotNil(t, signal)
	assert.Equal(t, "BTCUSDT", signal.Symbol)
	assert.True(t, signal.LossPercent.Equal(decimal.NewFromInt(6)))
}

func TestGenerateStopLossSignal_NoTriggerBelowThreshold(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		StopLossPercent: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromInt(10)},
	})

	// 入场价 100，当前价 95 → 亏损 = 5% < 10%
	signal := rm.GenerateStopLossSignal("BTCUSDT",
		decimal.NewFromInt(100), decimal.NewFromInt(95), decimal.NewFromInt(1))
	assert.Nil(t, signal)
}

func TestGenerateStopLossSignal_NoConfigForSymbol(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		StopLossPercent: map[string]decimal.Decimal{},
	})

	signal := rm.GenerateStopLossSignal("ETHUSDT",
		decimal.NewFromInt(100), decimal.NewFromInt(50), decimal.NewFromInt(1))
	assert.Nil(t, signal)
}

func TestGenerateStopLossSignal_ZeroEntryPrice(t *testing.T) {
	rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
	rm.SetConfig(RiskConfig{
		StopLossPercent: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromInt(5)},
	})

	signal := rm.GenerateStopLossSignal("BTCUSDT",
		decimal.Zero, decimal.NewFromInt(50), decimal.NewFromInt(1))
	assert.Nil(t, signal)
}

// --- PauseAllStrategies 测试 ---

func TestPauseAllStrategies_CallsPauserAndNotifies(t *testing.T) {
	pauser := &mockPauser{}
	rm, notifStore := newTestRiskManager(&mockRiskStore{}, pauser)

	err := rm.PauseAllStrategies()
	assert.NoError(t, err)
	assert.True(t, pauser.called)
	assert.Len(t, notifStore.notifications, 1)
	assert.Equal(t, string(notification.EventRiskAlert), notifStore.notifications[0].EventType)
}

func TestPauseAllStrategies_NilPauser(t *testing.T) {
	rm, notifStore := newTestRiskManager(&mockRiskStore{}, nil)

	err := rm.PauseAllStrategies()
	assert.NoError(t, err)
	// 通知仍应被发送。
	assert.Len(t, notifStore.notifications, 1)
}

// --- SaveConfig / LoadConfig 测试 ---

func TestSaveAndLoadConfig(t *testing.T) {
	store := &mockRiskStore{}
	rm, _ := newTestRiskManager(store, nil)

	original := RiskConfig{
		MaxOrderAmount:   decimal.NewFromInt(5000),
		MaxDailyLoss:     decimal.NewFromInt(1000),
		StopLossPercent:  map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(5.5)},
		MaxPositionRatio: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(0.3)},
	}
	rm.SetConfig(original)

	err := rm.SaveConfig()
	require.NoError(t, err)
	require.NotNil(t, store.config)

	// 创建一个新的 RiskManager 并从同一个 store 加载。
	rm2, _ := newTestRiskManager(store, nil)
	err = rm2.LoadConfig()
	require.NoError(t, err)

	loaded := rm2.GetConfig()
	assert.True(t, loaded.MaxOrderAmount.Equal(original.MaxOrderAmount))
	assert.True(t, loaded.MaxDailyLoss.Equal(original.MaxDailyLoss))
	assert.True(t, loaded.StopLossPercent["BTCUSDT"].Equal(original.StopLossPercent["BTCUSDT"]))
	assert.True(t, loaded.MaxPositionRatio["BTCUSDT"].Equal(original.MaxPositionRatio["BTCUSDT"]))
}

func TestSaveConfig_MarshalsProperly(t *testing.T) {
	store := &mockRiskStore{}
	rm, _ := newTestRiskManager(store, nil)
	rm.SetConfig(RiskConfig{
		MaxOrderAmount:   decimal.NewFromInt(100),
		MaxDailyLoss:     decimal.NewFromInt(50),
		StopLossPercent:  map[string]decimal.Decimal{"ETHUSDT": decimal.NewFromInt(3)},
		MaxPositionRatio: map[string]decimal.Decimal{"ETHUSDT": decimal.NewFromFloat(0.2)},
	})

	err := rm.SaveConfig()
	require.NoError(t, err)

	// 验证 JSON 字段有效。
	var slp map[string]decimal.Decimal
	err = json.Unmarshal(store.config.StopLossPercents, &slp)
	require.NoError(t, err)
	assert.True(t, slp["ETHUSDT"].Equal(decimal.NewFromInt(3)))

	var mpr map[string]decimal.Decimal
	err = json.Unmarshal(store.config.MaxPositionPercents, &mpr)
	require.NoError(t, err)
	assert.True(t, mpr["ETHUSDT"].Equal(decimal.NewFromFloat(0.2)))
}

// --- 属性测试 ---

// Feature: binance-trading-system, Property 13: 风控拒绝超限订单
// **Validates: Requirements 6.3, 6.7**
func TestProperty13_RiskRejectsOverLimitOrders(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 使用整数生成正数限制以避免浮点精度问题。
		maxOrderAmountInt := rapid.Int64Range(100, 100000).Draw(t, "maxOrderAmount")
		maxOrderAmount := decimal.NewFromInt(maxOrderAmountInt)

		maxRatioNum := rapid.Int64Range(1, 99).Draw(t, "maxRatioPercent")
		maxRatio := decimal.NewFromInt(maxRatioNum).Div(decimal.NewFromInt(100))

		totalAssetInt := rapid.Int64Range(1000, 1000000).Draw(t, "totalAssetValue")
		totalAssetValue := decimal.NewFromInt(totalAssetInt)

		symbol := "BTCUSDT"

		rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
		rm.SetConfig(RiskConfig{
			MaxOrderAmount:   maxOrderAmount,
			MaxPositionRatio: map[string]decimal.Decimal{symbol: maxRatio},
		})

		// 情况 1：金额超过最大订单金额 → 应被拒绝。
		overAmountInt := rapid.Int64Range(maxOrderAmountInt+1, maxOrderAmountInt+100000).Draw(t, "overAmount")
		overAmount := decimal.NewFromInt(overAmountInt)
		err := rm.CheckOrder(symbol, overAmount, totalAssetValue)
		assert.Error(t, err, "order exceeding max amount should be rejected")

		// 情况 2：金额在最大订单金额内但比例超限 → 应被拒绝。
		// 需要 amount/totalAssetValue > maxRatio，即 amount > maxRatio * totalAssetValue。
		ratioThreshold := maxRatio.Mul(totalAssetValue).IntPart() + 1
		// 确保金额也在最大订单金额内，以隔离比例检查。
		if ratioThreshold > 0 && decimal.NewFromInt(ratioThreshold).LessThanOrEqual(maxOrderAmount) {
			err = rm.CheckOrder(symbol, decimal.NewFromInt(ratioThreshold), totalAssetValue)
			assert.Error(t, err, "order exceeding position ratio should be rejected")
		}

		// 情况 3：金额在两个限制内 → 应被接受。
		safeAmount := maxOrderAmount
		safeRatioAmount := maxRatio.Mul(totalAssetValue).Truncate(0)
		if safeRatioAmount.LessThan(safeAmount) {
			safeAmount = safeRatioAmount
		}
		if safeAmount.IsPositive() {
			err = rm.CheckOrder(symbol, safeAmount, totalAssetValue)
			assert.NoError(t, err, "order within all limits should be accepted")
		}
	})
}

// Feature: binance-trading-system, Property 14: 每日亏损阈值触发策略暂停
// **Validates: Requirements 6.4**
func TestProperty14_DailyLossThresholdTriggersStrategyPause(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxDailyLossInt := rapid.Int64Range(100, 50000).Draw(t, "maxDailyLoss")
		maxDailyLoss := decimal.NewFromInt(maxDailyLossInt)

		rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
		rm.SetConfig(RiskConfig{MaxDailyLoss: maxDailyLoss})

		now := time.Now()

		// 生成随机交易序列。
		numTrades := rapid.IntRange(1, 20).Draw(t, "numTrades")
		trades := make([]model.Trade, numTrades)
		for i := 0; i < numTrades; i++ {
			side := "BUY"
			if rapid.Bool().Draw(t, "isSell") {
				side = "SELL"
			}
			amountInt := rapid.Int64Range(1, 10000).Draw(t, "tradeAmount")
			feeInt := rapid.Int64Range(0, 100).Draw(t, "tradeFee")
			trades[i] = model.Trade{
				Side:       side,
				Amount:     decimal.NewFromInt(amountInt),
				Fee:        decimal.NewFromInt(feeInt),
				ExecutedAt: now,
			}
		}

		// 计算预期每日亏损：sum(买入金额) - sum(卖出金额) + sum(所有手续费)。
		expectedLoss := decimal.Zero
		for _, tr := range trades {
			switch tr.Side {
			case "BUY":
				expectedLoss = expectedLoss.Add(tr.Amount)
			case "SELL":
				expectedLoss = expectedLoss.Sub(tr.Amount)
			}
			expectedLoss = expectedLoss.Add(tr.Fee)
		}

		shouldPause, dailyLoss := rm.CheckDailyLoss(trades)
		assert.True(t, dailyLoss.Equal(expectedLoss), "daily loss calculation mismatch: got %s, want %s", dailyLoss.String(), expectedLoss.String())

		if expectedLoss.GreaterThanOrEqual(maxDailyLoss) {
			assert.True(t, shouldPause, "should pause when daily loss %s >= max %s", expectedLoss.String(), maxDailyLoss.String())
		} else {
			assert.False(t, shouldPause, "should not pause when daily loss %s < max %s", expectedLoss.String(), maxDailyLoss.String())
		}
	})
}

// Feature: binance-trading-system, Property 15: 止损信号在阈值触发
// **Validates: Requirements 6.5**
func TestProperty15_StopLossSignalAtThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 使用整数生成正数入场价和止损阈值。
		entryPriceInt := rapid.Int64Range(100, 1000000).Draw(t, "entryPrice")
		entryPrice := decimal.NewFromInt(entryPriceInt)

		stopLossPctInt := rapid.Int64Range(1, 50).Draw(t, "stopLossPercent")
		stopLossPct := decimal.NewFromInt(stopLossPctInt)

		quantityInt := rapid.Int64Range(1, 1000).Draw(t, "quantity")
		quantity := decimal.NewFromInt(quantityInt)

		// 生成可能触发或不触发止损的当前价格。
		// currentPrice 范围从 1 到 entryPrice（我们只关心亏损和无亏损场景）。
		currentPriceInt := rapid.Int64Range(1, entryPriceInt).Draw(t, "currentPrice")
		currentPrice := decimal.NewFromInt(currentPriceInt)

		symbol := "BTCUSDT"

		rm, _ := newTestRiskManager(&mockRiskStore{}, nil)
		rm.SetConfig(RiskConfig{
			StopLossPercent: map[string]decimal.Decimal{symbol: stopLossPct},
		})

		signal := rm.GenerateStopLossSignal(symbol, entryPrice, currentPrice, quantity)

		// lossPercent = (entryPrice - currentPrice) / entryPrice * 100
		lossPercent := entryPrice.Sub(currentPrice).Div(entryPrice).Mul(decimal.NewFromInt(100))

		if lossPercent.GreaterThanOrEqual(stopLossPct) {
			require.NotNil(t, signal, "signal should be generated when lossPercent %s >= threshold %s", lossPercent.String(), stopLossPct.String())
			assert.Equal(t, symbol, signal.Symbol)
			assert.True(t, signal.LossPercent.Equal(lossPercent), "signal lossPercent mismatch")
			assert.True(t, signal.EntryPrice.Equal(entryPrice))
			assert.True(t, signal.CurrentPrice.Equal(currentPrice))
			assert.True(t, signal.Quantity.Equal(quantity))
		} else {
			assert.Nil(t, signal, "signal should NOT be generated when lossPercent %s < threshold %s", lossPercent.String(), stopLossPct.String())
		}
	})
}
