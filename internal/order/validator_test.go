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

// helper to build a standard BTCUSDT symbol with typical Binance filters.
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
		Quantity: decimal.NewFromFloat(0.000001), // below 0.00001
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
		Quantity: decimal.NewFromFloat(10000), // above 9000
		Price:    decimal.NewFromFloat(50000.00),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "exceeds maximum")
}

func TestValidate_QuantityStepSize(t *testing.T) {
	v := newValidator()
	// 0.000015 - 0.00001 = 0.000005, which is not a multiple of 0.00001
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
		Price:    decimal.NewFromFloat(0.001), // below 0.01
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
		Price:    decimal.NewFromFloat(2000000), // above 1000000
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
		Price:    decimal.RequireFromString("50000.005"), // not a multiple of 0.01
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Contains(t, appErr.Message, "tick size")
}

func TestValidate_NotionalBelowMin(t *testing.T) {
	v := newValidator()
	// 0.00001 * 100 = 0.001 USDT, below 10 USDT
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
	// quantity below min AND notional below min
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
	// Should contain both quantity and notional errors
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
	// BTCUSDT rules from testExchangeInfo():
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

			// Generate quantity as integer multiples of stepSize within [minQty, maxQty].
			// minQty = 0.00001 = 1 step, maxQty = 9000 = 900000000 steps
			qtySteps := rapid.Int64Range(1, 900000000).Draw(t, "qtySteps")
			qty := stepSize.Mul(decimal.NewFromInt(qtySteps))

			// We need price * qty >= minNotional (10) and price in [minPrice, maxPrice].
			// Compute minimum price ticks to satisfy notional: ceil(minNotional / qty / tickSize)
			minTicksForNotional := minNotional.Div(qty).Div(tickSize).Ceil().IntPart()
			if minTicksForNotional < 1 {
				minTicksForNotional = 1
			}
			maxTicks := maxPrice.Div(tickSize).IntPart() // 1000000 / 0.01 = 100000000
			if minTicksForNotional > maxTicks {
				// Skip: impossible to satisfy notional with this qty within price range
				t.Skip("cannot satisfy notional constraint")
			}

			priceTicks := rapid.Int64Range(minTicksForNotional, maxTicks).Draw(t, "priceTicks")
			price := tickSize.Mul(decimal.NewFromInt(priceTicks))

			// Sanity: verify our generated values meet all constraints
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

			// Generate a quantity that is below minQty but still positive.
			// minQty = 0.00001 = 1e-5, so generate fractions of it.
			// Use a divisor > 1 to ensure qty < minQty.
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

			// Generate quantity above maxQty, aligned to step.
			// maxQty = 9000 = 900000000 steps. Generate steps above that.
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

			// Generate a valid base quantity, then add a fractional offset that breaks step alignment.
			// stepSize = 0.00001. Add half a step to break alignment.
			baseSteps := rapid.Int64Range(1, 100000).Draw(t, "baseSteps")
			qty := stepSize.Mul(decimal.NewFromInt(baseSteps)).Add(decimal.RequireFromString("0.000005"))

			// Use a high price so notional is satisfied
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

			// Generate a price below minPrice (0.01).
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

			// Generate price above maxPrice, aligned to tick.
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

			// Generate a valid base price, then add half a tick to break alignment.
			// tickSize = 0.01. Add 0.005 to break alignment.
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

			// Generate valid qty and price individually, but ensure notional < minNotional (10).
			// Use small qty and small price so price*qty < 10.
			// qty in [minQty, 0.001] (step-aligned), price in [minPrice, some low value] (tick-aligned)
			qtySteps := rapid.Int64Range(1, 100).Draw(t, "qtySteps") // max 0.001
			qty := stepSize.Mul(decimal.NewFromInt(qtySteps))

			// We need price * qty < 10. So price < 10 / qty.
			maxPriceForLowNotional := minNotional.Div(qty).Sub(tickSize) // ensure strictly below
			if maxPriceForLowNotional.LessThan(minPrice) {
				t.Skip("cannot generate low notional with this qty")
			}
			maxPriceTicks := maxPriceForLowNotional.Div(tickSize).IntPart()
			if maxPriceTicks < 1 {
				t.Skip("cannot generate low notional price ticks")
			}
			priceTicks := rapid.Int64Range(1, maxPriceTicks).Draw(t, "priceTicks")
			price := tickSize.Mul(decimal.NewFromInt(priceTicks))

			// Verify notional is indeed below min
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
