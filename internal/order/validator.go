package order

import (
	"fmt"
	"money-loves-me/internal/errors"
	"money-loves-me/pkg/binance"
	"strings"

	"github.com/shopspring/decimal"
)

// ExchangeInfoProvider abstracts access to exchange trading rules, enabling testing.
type ExchangeInfoProvider interface {
	GetSymbolInfo(symbol string) (*binance.SymbolInfo, bool)
}

// MapExchangeInfoProvider implements ExchangeInfoProvider using an in-memory map.
type MapExchangeInfoProvider struct {
	symbols map[string]binance.SymbolInfo
}

// NewMapExchangeInfoProvider builds a provider from an ExchangeInfo response.
func NewMapExchangeInfoProvider(info *binance.ExchangeInfo) *MapExchangeInfoProvider {
	m := make(map[string]binance.SymbolInfo, len(info.Symbols))
	for _, s := range info.Symbols {
		m[s.Symbol] = s
	}
	return &MapExchangeInfoProvider{symbols: m}
}

// GetSymbolInfo returns the SymbolInfo for the given symbol.
func (p *MapExchangeInfoProvider) GetSymbolInfo(symbol string) (*binance.SymbolInfo, bool) {
	info, ok := p.symbols[symbol]
	if !ok {
		return nil, false
	}
	return &info, true
}

// OrderValidator validates order parameters against exchange trading rules.
type OrderValidator struct {
	provider ExchangeInfoProvider
}

// NewOrderValidator creates a new OrderValidator with the given exchange info provider.
func NewOrderValidator(provider ExchangeInfoProvider) *OrderValidator {
	return &OrderValidator{provider: provider}
}

// ValidationError holds a list of individual validation failures.
type ValidationError struct {
	Errors []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("order validation failed: %s", strings.Join(v.Errors, "; "))
}

// Add appends a validation failure message.
func (v *ValidationError) Add(msg string) {
	v.Errors = append(v.Errors, msg)
}

// HasErrors returns true if any validation failures were recorded.
func (v *ValidationError) HasErrors() bool {
	return len(v.Errors) > 0
}

// Validate checks the order request against the exchange trading rules.
// It returns a structured AppError (ErrValidation) wrapping a ValidationError on failure.
func (v *OrderValidator) Validate(req binance.CreateOrderRequest) error {
	// Look up symbol
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

	// Validate LOT_SIZE filter (quantity)
	if filter, found := findFilter(info.Filters, "LOT_SIZE"); found {
		validateLotSize(ve, req.Quantity, filter)
	}

	// Validate PRICE_FILTER (price)
	if filter, found := findFilter(info.Filters, "PRICE_FILTER"); found {
		validatePriceFilter(ve, req.Price, filter)
	}

	// Validate MIN_NOTIONAL (price * quantity)
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

// findFilter returns the first filter matching the given type.
func findFilter(filters []binance.SymbolFilter, filterType string) (binance.SymbolFilter, bool) {
	for _, f := range filters {
		if f.FilterType == filterType {
			return f, true
		}
	}
	return binance.SymbolFilter{}, false
}

// validateLotSize checks quantity against MinQty, MaxQty, and StepSize.
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

// validatePriceFilter checks price against MinPrice, MaxPrice, and TickSize.
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

// validateMinNotional checks that price * quantity >= MinNotional.
func validateMinNotional(ve *ValidationError, price, qty decimal.Decimal, f binance.SymbolFilter) {
	minNotional, _ := decimal.NewFromString(f.MinNotional)
	notional := price.Mul(qty)
	if notional.LessThan(minNotional) {
		ve.Add(fmt.Sprintf("notional value %s is below minimum %s", notional, minNotional))
	}
}

// fitsStep returns true if (value - base) is an exact multiple of step.
// This is the standard Binance check: (value - base) % step == 0.
func fitsStep(value, base, step decimal.Decimal) bool {
	diff := value.Sub(base)
	if diff.IsNegative() {
		return false
	}
	remainder := diff.Mod(step)
	return remainder.IsZero()
}
