package order

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/pkg/binance"
)

// 辅助函数，构建一个具有典型 Binance 过滤器的标准 BTCUSDT 交易对。
func testExchangeInfo() *binance.ExchangeInfo {
	return &binance.ExchangeInfo{
		Symbols: []binance.SymbolInfo{
			{
				Symbol:     "BTCUSDT",
				Status:     "TRADING",
				BaseAsset:  "BTC",
				QuoteAsset: "USDT",
				Filters: []binance.SymbolFilter{
					{
						FilterType: "LOT_SIZE",
						MinQty:     "0.00001",
						MaxQty:     "9000.00000",
						StepSize:   "0.00001",
					},
					{
						FilterType: "PRICE_FILTER",
						MinPrice:   "0.01",
						MaxPrice:   "1000000.00",
						TickSize:   "0.01",
					},
					{
						FilterType:  "MIN_NOTIONAL",
						MinNotional: "10.00",
					},
				},
			},
		},
	}
}

func newValidator() *OrderValidator {
	return NewOrderValidator(NewMapExchangeInfoProvider(testExchangeInfo()))
}

func TestValidate_ValidOrder(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.001),
		Price:    decimal.NewFromFloat(50000.00),
	})
	assert.NoError(t, err)
}

func TestValidate_SymbolNotFound(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "FOOBAR",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(1),
		Price:    decimal.NewFromFloat(100),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.ErrValidation, appErr.Code)
	assert.Contains(t, appErr.Message, "does not exist")
}

func TestValidate_QuantityBelowMin(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.000001), // 低于 0.00001
		Price:    decimal.NewFromFloat(50000.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "below minimum")
}

func TestValidate_QuantityAboveMax(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(10000), // 超过 9000
		Price:    decimal.NewFromFloat(50000.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "exceeds maximum")
}

func TestValidate_QuantityStepSize(t *testing.T) {
	v := newValidator()
	// 0.000015 - 0.00001 = 0.000005，不是 0.00001 的倍数
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.RequireFromString("0.000015"),
		Price:    decimal.NewFromFloat(50000.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "step size")
}

func TestValidate_PriceBelowMin(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(1),
		Price:    decimal.NewFromFloat(0.001), // 低于 0.01
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "below minimum")
}

func TestValidate_PriceAboveMax(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.001),
		Price:    decimal.NewFromFloat(2000000), // 超过 1000000
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "exceeds maximum")
}

func TestValidate_PriceTickSize(t *testing.T) {
	v := newValidator()
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.001),
		Price:    decimal.RequireFromString("50000.005"), // 不是 0.01 的倍数
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "tick size")
}

func TestValidate_NotionalBelowMin(t *testing.T) {
	v := newValidator()
	// 0.00001 * 100 = 0.001 USDT，低于 10 USDT
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.00001),
		Price:    decimal.NewFromFloat(100.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "notional")
}

func TestValidate_MultipleErrors(t *testing.T) {
	v := newValidator()
	// 数量低于最小值 且 名义价值低于最小值
	err := v.Validate(binance.CreateOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: decimal.NewFromFloat(0.000001),
		Price:    decimal.NewFromFloat(1.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	// 应同时包含数量和名义价值的错误
	var ve *ValidationError
	require.ErrorAs(t, appErr.Cause, &ve)
	assert.GreaterOrEqual(t, len(ve.Errors), 2)
}

func TestMapExchangeInfoProvider_FromExchangeInfo(t *testing.T) {
	info := testExchangeInfo()
	provider := NewMapExchangeInfoProvider(info)

	si, ok := provider.GetSymbolInfo("BTCUSDT")
	assert.True(t, ok)
	assert.Equal(t, "BTCUSDT", si.Symbol)

	_, ok = provider.GetSymbolInfo("NONEXIST")
	assert.False(t, ok)
}

// Feature: binance-trading-system, Property 4: 订单参数验证正确性
// **Validates: Requirements 3.2**

func TestProperty4_OrderParameterValidation(t *testing.T) {
	// testExchangeInfo() 中的 BTCUSDT 规则：
	//   LOT_SIZE:     min=0.00001, max=9000, step=0.00001
	//   PRICE_FILTER: min=0.01, max=1000000, tick=0.01
	//   MIN_NOTIONAL: min=10

	stepSize := decimal.RequireFromString("0.00001")
	minQty := decimal.RequireFromString("0.00001")
	maxQty := decimal.RequireFromString("9000")
	tickSize := decimal.RequireFromString("0.01")
	minPrice := decimal.RequireFromString("0.01")
	maxPrice := decimal.RequireFromString("1000000")
	minNotional := decimal.RequireFromString("10")

	t.Run("valid_orders_pass_validation", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成数量为 stepSize 的整数倍，范围在 [minQty, maxQty] 内。
			// minQty = 0.00001 = 1 步, maxQty = 9000 = 900000000 步
			qtySteps := rapid.Int64Range(1, 900000000).Draw(t, "qtySteps")
			qty := stepSize.Mul(decimal.NewFromInt(qtySteps))

			// 需要 price * qty >= minNotional (10) 且 price 在 [minPrice, maxPrice] 范围内。
			// 计算满足名义价值的最小价格刻度数：ceil(minNotional / qty / tickSize)
			minTicksForNotional := minNotional.Div(qty).Div(tickSize).Ceil().IntPart()
			if minTicksForNotional < 1 {
				minTicksForNotional = 1
			}
			maxTicks := maxPrice.Div(tickSize).IntPart() // 1000000 / 0.01 = 100000000
			if minTicksForNotional > maxTicks {
				// 跳过：在价格范围内无法满足名义价值约束
				t.Skip("cannot satisfy notional constraint")
			}

			priceTicks := rapid.Int64Range(minTicksForNotional, maxTicks).Draw(t, "priceTicks")
			price := tickSize.Mul(decimal.NewFromInt(priceTicks))

			// 健全性检查：验证生成的值满足所有约束
			require.True(t, qty.GreaterThanOrEqual(minQty))
			require.True(t, qty.LessThanOrEqual(maxQty))
			require.True(t, price.GreaterThanOrEqual(minPrice))
			require.True(t, price.LessThanOrEqual(maxPrice))
			require.True(t, price.Mul(qty).GreaterThanOrEqual(minNotional))

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			assert.NoError(t, err, "valid order should pass: qty=%s price=%s notional=%s", qty, price, price.Mul(qty))
		})
	})

	t.Run("invalid_quantity_below_min_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成低于 minQty 但仍为正数的数量。
			// minQty = 0.00001 = 1e-5，因此生成其分数。
			// 使用大于 1 的除数确保 qty < minQty。
			divisor := rapid.Int64Range(2, 100).Draw(t, "divisor")
			qty := minQty.Div(decimal.NewFromInt(divisor))

			price := decimal.NewFromFloat(50000.00)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_quantity_above_max_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成超过 maxQty 且对齐步长的数量。
			// maxQty = 9000 = 900000000 步。生成超过该值的步数。
			extraSteps := rapid.Int64Range(1, 1000000).Draw(t, "extraSteps")
			qty := maxQty.Add(stepSize.Mul(decimal.NewFromInt(extraSteps)))

			price := decimal.NewFromFloat(50000.00)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_quantity_wrong_step_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成一个有效的基础数量，然后添加一个破坏步长对齐的小数偏移。
			// stepSize = 0.00001。添加半个步长来破坏对齐。
			baseSteps := rapid.Int64Range(1, 100000).Draw(t, "baseSteps")
			qty := stepSize.Mul(decimal.NewFromInt(baseSteps)).Add(decimal.RequireFromString("0.000005"))

			// 使用较高的价格以满足名义价值要求
			price := decimal.NewFromFloat(50000.00)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_price_below_min_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成低于 minPrice (0.01) 的价格。
			divisor := rapid.Int64Range(2, 100).Draw(t, "divisor")
			price := minPrice.Div(decimal.NewFromInt(divisor))

			qty := decimal.NewFromFloat(1.0)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_price_above_max_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成超过 maxPrice 且对齐刻度的价格。
			extraTicks := rapid.Int64Range(1, 1000000).Draw(t, "extraTicks")
			price := maxPrice.Add(tickSize.Mul(decimal.NewFromInt(extraTicks)))

			qty := decimal.NewFromFloat(0.001)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_price_wrong_tick_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成一个有效的基础价格，然后添加半个刻度来破坏对齐。
			// tickSize = 0.01。添加 0.005 来破坏对齐。
			baseTicks := rapid.Int64Range(1, 100000).Draw(t, "baseTicks")
			price := tickSize.Mul(decimal.NewFromInt(baseTicks)).Add(decimal.RequireFromString("0.005"))

			qty := decimal.NewFromFloat(0.01)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})

	t.Run("invalid_notional_below_min_rejected", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			v := newValidator()

			// 生成单独有效的 qty 和 price，但确保 notional < minNotional (10)。
			// 使用较小的 qty 和较小的 price 使得 price*qty < 10。
			// qty 在 [minQty, 0.001]（步长对齐），price 在 [minPrice, 某个较低值]（刻度对齐）
			qtySteps := rapid.Int64Range(1, 100).Draw(t, "qtySteps") // 最大 0.001
			qty := stepSize.Mul(decimal.NewFromInt(qtySteps))

			// 需要 price * qty < 10。所以 price < 10 / qty。
			maxPriceForLowNotional := minNotional.Div(qty).Sub(tickSize) // 确保严格低于
			if maxPriceForLowNotional.LessThan(minPrice) {
				t.Skip("cannot generate low notional with this qty")
			}
			maxPriceTicks := maxPriceForLowNotional.Div(tickSize).IntPart()
			if maxPriceTicks < 1 {
				t.Skip("cannot generate low notional price ticks")
			}
			priceTicks := rapid.Int64Range(1, maxPriceTicks).Draw(t, "priceTicks")
			price := tickSize.Mul(decimal.NewFromInt(priceTicks))

			// 验证名义价值确实低于最小值
			notional := price.Mul(qty)
			require.True(t, notional.LessThan(minNotional), "notional %s should be < %s", notional, minNotional)

			err := v.Validate(binance.CreateOrderRequest{
				Symbol:   "BTCUSDT",
				Side:     "BUY",
				Type:     "LIMIT",
				Quantity: qty,
				Price:    price,
			})
			require.Error(t, err)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.ErrValidation, appErr.Code)
		})
	})
}
