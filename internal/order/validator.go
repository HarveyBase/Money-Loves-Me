package order

import (
	"fmt"
	"money-loves-me/internal/errors"
	"money-loves-me/pkg/binance"
	"strings"

	"github.com/shopspring/decimal"
)

// ExchangeInfoProvider 抽象了对交易所交易规则的访问，便于测试。
type ExchangeInfoProvider interface {
	GetSymbolInfo(symbol string) (*binance.SymbolInfo, bool)
}

// MapExchangeInfoProvider 使用内存中的 map 实现 ExchangeInfoProvider。
type MapExchangeInfoProvider struct {
	symbols map[string]binance.SymbolInfo
}

// NewMapExchangeInfoProvider 根据 ExchangeInfo 响应构建一个 provider。
func NewMapExchangeInfoProvider(info *binance.ExchangeInfo) *MapExchangeInfoProvider {
	m := make(map[string]binance.SymbolInfo, len(info.Symbols))
	for _, s := range info.Symbols {
		m[s.Symbol] = s
	}
	return &MapExchangeInfoProvider{symbols: m}
}

// GetSymbolInfo 返回给定交易对的 SymbolInfo。
func (p *MapExchangeInfoProvider) GetSymbolInfo(symbol string) (*binance.SymbolInfo, bool) {
	info, ok := p.symbols[symbol]
	if !ok {
		return nil, false
	}
	return &info, true
}

// OrderValidator 根据交易所交易规则验证订单参数。
type OrderValidator struct {
	provider ExchangeInfoProvider
}

// NewOrderValidator 使用给定的交易所信息 provider 创建一个新的 OrderValidator。
func NewOrderValidator(provider ExchangeInfoProvider) *OrderValidator {
	return &OrderValidator{provider: provider}
}

// ValidationError 保存一组验证失败信息。
type ValidationError struct {
	Errors []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("order validation failed: %s", strings.Join(v.Errors, "; "))
}

// Add 追加一条验证失败消息。
func (v *ValidationError) Add(msg string) {
	v.Errors = append(v.Errors, msg)
}

// HasErrors 如果记录了任何验证失败则返回 true。
func (v *ValidationError) HasErrors() bool {
	return len(v.Errors) > 0
}

// Validate 根据交易所交易规则检查订单请求。
// 验证失败时返回一个结构化的 AppError（ErrValidation），其中包装了 ValidationError。
func (v *OrderValidator) Validate(req binance.CreateOrderRequest) error {
	// 查找交易对
	info, ok := v.provider.GetSymbolInfo(req.Symbol)
	if !ok {
		return errors.NewAppError(
			errors.ErrValidation,
			fmt.Sprintf("symbol %q does not exist", req.Symbol),
			"OrderValidator",
			nil,
		)
	}

	ve := &ValidationError{}

	// 验证 LOT_SIZE 过滤器（数量）
	if filter, found := findFilter(info.Filters, "LOT_SIZE"); found {
		validateLotSize(ve, req.Quantity, filter)
	}

	// 验证 PRICE_FILTER（价格）
	if filter, found := findFilter(info.Filters, "PRICE_FILTER"); found {
		validatePriceFilter(ve, req.Price, filter)
	}

	// 验证 MIN_NOTIONAL（价格 * 数量）
	if filter, found := findFilter(info.Filters, "MIN_NOTIONAL"); found {
		validateMinNotional(ve, req.Price, req.Quantity, filter)
	}

	if ve.HasErrors() {
		return errors.NewAppError(
			errors.ErrValidation,
			ve.Error(),
			"OrderValidator",
			ve,
		)
	}
	return nil
}

// findFilter 返回第一个匹配给定类型的过滤器。
func findFilter(filters []binance.SymbolFilter, filterType string) (binance.SymbolFilter, bool) {
	for _, f := range filters {
		if f.FilterType == filterType {
			return f, true
		}
	}
	return binance.SymbolFilter{}, false
}

// validateLotSize 检查数量是否符合 MinQty、MaxQty 和 StepSize。
func validateLotSize(ve *ValidationError, qty decimal.Decimal, f binance.SymbolFilter) {
	minQty, _ := decimal.NewFromString(f.MinQty)
	maxQty, _ := decimal.NewFromString(f.MaxQty)
	stepSize, _ := decimal.NewFromString(f.StepSize)

	if qty.LessThan(minQty) {
		ve.Add(fmt.Sprintf("quantity %s is below minimum %s", qty, minQty))
	}
	if !maxQty.IsZero() && qty.GreaterThan(maxQty) {
		ve.Add(fmt.Sprintf("quantity %s exceeds maximum %s", qty, maxQty))
	}
	if !stepSize.IsZero() && !fitsStep(qty, minQty, stepSize) {
		ve.Add(fmt.Sprintf("quantity %s does not conform to step size %s", qty, stepSize))
	}
}

// validatePriceFilter 检查价格是否符合 MinPrice、MaxPrice 和 TickSize。
func validatePriceFilter(ve *ValidationError, price decimal.Decimal, f binance.SymbolFilter) {
	minPrice, _ := decimal.NewFromString(f.MinPrice)
	maxPrice, _ := decimal.NewFromString(f.MaxPrice)
	tickSize, _ := decimal.NewFromString(f.TickSize)

	if !minPrice.IsZero() && price.LessThan(minPrice) {
		ve.Add(fmt.Sprintf("price %s is below minimum %s", price, minPrice))
	}
	if !maxPrice.IsZero() && price.GreaterThan(maxPrice) {
		ve.Add(fmt.Sprintf("price %s exceeds maximum %s", price, maxPrice))
	}
	if !tickSize.IsZero() && !fitsStep(price, minPrice, tickSize) {
		ve.Add(fmt.Sprintf("price %s does not conform to tick size %s", price, tickSize))
	}
}

// validateMinNotional 检查 价格 * 数量 >= MinNotional。
func validateMinNotional(ve *ValidationError, price, qty decimal.Decimal, f binance.SymbolFilter) {
	minNotional, _ := decimal.NewFromString(f.MinNotional)
	notional := price.Mul(qty)
	if notional.LessThan(minNotional) {
		ve.Add(fmt.Sprintf("notional value %s is below minimum %s", notional, minNotional))
	}
}

// fitsStep 返回 (value - base) 是否是 step 的精确倍数。
// 这是标准的 Binance 检查：(value - base) % step == 0。
func fitsStep(value, base, step decimal.Decimal) bool {
	diff := value.Sub(base)
	if diff.IsNegative() {
		return false
	}
	remainder := diff.Mod(step)
	return remainder.IsZero()
}
