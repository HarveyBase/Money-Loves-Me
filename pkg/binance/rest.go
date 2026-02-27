package binance

import (
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	apperrors "money-loves-me/internal/errors"

	"github.com/shopspring/decimal"
)

// GetKlines fetches candlestick (kline) data for the given symbol and interval.
// startTime and endTime are Unix millisecond timestamps; pass 0 to omit.
func (c *BinanceClient) GetKlines(symbol, interval string, startTime, endTime int64) ([]Kline, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", "1000")
	if startTime > 0 {
		params.Set("startTime", strconv.FormatInt(startTime, 10))
	}
	if endTime > 0 {
		params.Set("endTime", strconv.FormatInt(endTime, 10))
	}

	body, err := c.doPublicRequest("/api/v3/klines", params)
	if err != nil {
		return nil, err
	}

	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse klines", "binance", err)
	}

	klines := make([]Kline, 0, len(raw))
	for _, r := range raw {
		if len(r) < 7 {
			continue
		}
		k, err := parseKlineRow(r)
		if err != nil {
			continue
		}
		klines = append(klines, k)
	}
	return klines, nil
}

// CreateOrder submits a new order to Binance.
func (c *BinanceClient) CreateOrder(req CreateOrderRequest) (*OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", req.Side)
	params.Set("type", req.Type)
	params.Set("quantity", req.Quantity.String())

	if req.TimeInForce != "" {
		params.Set("timeInForce", req.TimeInForce)
	}
	if !req.Price.IsZero() {
		params.Set("price", req.Price.String())
	}
	if !req.StopPrice.IsZero() {
		params.Set("stopPrice", req.StopPrice.String())
	}

	body, err := c.doSignedRequest("POST", "/api/v3/order", params)
	if err != nil {
		return nil, err
	}

	var resp OrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse order response", "binance", err)
	}
	return &resp, nil
}

// CancelOrder cancels an active order on Binance.
func (c *BinanceClient) CancelOrder(symbol string, orderID int64) (*OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	body, err := c.doSignedRequest("DELETE", "/api/v3/order", params)
	if err != nil {
		return nil, err
	}

	var resp OrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse cancel response", "binance", err)
	}
	return &resp, nil
}

// GetAccountInfo retrieves the current account information (balances, permissions).
func (c *BinanceClient) GetAccountInfo() (*AccountInfo, error) {
	body, err := c.doSignedRequest("GET", "/api/v3/account", nil)
	if err != nil {
		return nil, err
	}

	var info AccountInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse account info", "binance", err)
	}
	return &info, nil
}

// GetExchangeInfo retrieves the exchange trading rules and symbol information.
func (c *BinanceClient) GetExchangeInfo() (*ExchangeInfo, error) {
	body, err := c.doPublicRequest("/api/v3/exchangeInfo", nil)
	if err != nil {
		return nil, err
	}

	var info ExchangeInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse exchange info", "binance", err)
	}
	return &info, nil
}

// GetOrderBook retrieves the order book depth for a symbol.
func (c *BinanceClient) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	body, err := c.doPublicRequest("/api/v3/depth", params)
	if err != nil {
		return nil, err
	}

	var raw binanceOrderBookRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI, "failed to parse order book", "binance", err)
	}

	book := &OrderBook{
		Symbol:     symbol,
		Bids:       parsePriceLevels(raw.Bids),
		Asks:       parsePriceLevels(raw.Asks),
		UpdateTime: time.Now(),
	}
	return book, nil
}

// --- helpers ---

func parseKlineRow(r []interface{}) (Kline, error) {
	openTime, err := toInt64(r[0])
	if err != nil {
		return Kline{}, err
	}
	closeTime, err := toInt64(r[6])
	if err != nil {
		return Kline{}, err
	}
	open, err := toDecimal(r[1])
	if err != nil {
		return Kline{}, err
	}
	high, err := toDecimal(r[2])
	if err != nil {
		return Kline{}, err
	}
	low, err := toDecimal(r[3])
	if err != nil {
		return Kline{}, err
	}
	cl, err := toDecimal(r[4])
	if err != nil {
		return Kline{}, err
	}
	vol, err := toDecimal(r[5])
	if err != nil {
		return Kline{}, err
	}

	return Kline{
		OpenTime:  time.UnixMilli(openTime),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     cl,
		Volume:    vol,
		CloseTime: time.UnixMilli(closeTime),
	}, nil
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case json.Number:
		return n.Int64()
	default:
		return 0, apperrors.NewAppError(apperrors.ErrBinanceAPI, "unexpected numeric type", "binance", nil)
	}
}

func toDecimal(v interface{}) (decimal.Decimal, error) {
	switch s := v.(type) {
	case string:
		return decimal.NewFromString(s)
	case float64:
		return decimal.NewFromFloat(s), nil
	default:
		return decimal.Zero, apperrors.NewAppError(apperrors.ErrBinanceAPI, "unexpected value type", "binance", nil)
	}
}

func parsePriceLevels(raw [][]string) []PriceLevel {
	levels := make([]PriceLevel, 0, len(raw))
	for _, entry := range raw {
		if len(entry) < 2 {
			continue
		}
		price, err := decimal.NewFromString(entry[0])
		if err != nil {
			continue
		}
		qty, err := decimal.NewFromString(entry[1])
		if err != nil {
			continue
		}
		levels = append(levels, PriceLevel{Price: price, Quantity: qty})
	}
	return levels
}
