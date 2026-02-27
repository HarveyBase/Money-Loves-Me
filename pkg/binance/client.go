package binance

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	apperrors "money-loves-me/internal/errors"
)

const (
	defaultBaseURL = "https://api.binance.com"
	defaultWsURL   = "wss://stream.binance.com:9443"

	// Rate limit: 1200 requests per minute = 20 per second.
	rateLimitPerSec = 20
)

// BinanceClient is the main entry point for interacting with the Binance API.
type BinanceClient struct {
	apiKey     string
	secretKey  string
	baseURL    string
	wsURL      string
	httpClient *http.Client
	signer     *HMACSigner
	limiter    *rateLimiter
}

// rateLimiter implements a simple token-bucket style rate limiter.
type rateLimiter struct {
	mu     sync.Mutex
	tokens int
	max    int
	ticker *time.Ticker
	stopCh chan struct{}
}

func newRateLimiter(ratePerSec int) *rateLimiter {
	rl := &rateLimiter{
		tokens: ratePerSec,
		max:    ratePerSec,
		stopCh: make(chan struct{}),
	}
	rl.ticker = time.NewTicker(time.Second)
	go rl.refill()
	return rl
}

func (rl *rateLimiter) refill() {
	for {
		select {
		case <-rl.ticker.C:
			rl.mu.Lock()
			rl.tokens = rl.max
			rl.mu.Unlock()
		case <-rl.stopCh:
			rl.ticker.Stop()
			return
		}
	}
}

// Wait blocks until a token is available.
func (rl *rateLimiter) Wait() {
	for {
		rl.mu.Lock()
		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return
		}
		rl.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
}

// Stop shuts down the rate limiter goroutine.
func (rl *rateLimiter) Stop() {
	close(rl.stopCh)
}

// ClientOption allows customising the BinanceClient.
type ClientOption func(*BinanceClient)

// WithBaseURL overrides the default REST base URL.
func WithBaseURL(u string) ClientOption {
	return func(c *BinanceClient) { c.baseURL = u }
}

// WithWsURL overrides the default WebSocket URL.
func WithWsURL(u string) ClientOption {
	return func(c *BinanceClient) { c.wsURL = u }
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *BinanceClient) { c.httpClient = hc }
}

// NewBinanceClient creates a new BinanceClient with the given credentials.
func NewBinanceClient(apiKey, secretKey string, opts ...ClientOption) *BinanceClient {
	c := &BinanceClient{
		apiKey:     apiKey,
		secretKey:  secretKey,
		baseURL:    defaultBaseURL,
		wsURL:      defaultWsURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		signer:     NewHMACSigner([]byte(secretKey)),
		limiter:    newRateLimiter(rateLimitPerSec),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Close releases resources held by the client.
func (c *BinanceClient) Close() {
	c.limiter.Stop()
}

// doPublicRequest performs an unauthenticated GET request.
func (c *BinanceClient) doPublicRequest(path string, params url.Values) ([]byte, error) {
	c.limiter.Wait()

	fullURL := c.baseURL + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrNetwork, "request failed", "binance", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrNetwork, "failed to read response", "binance", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewAppError(
			apperrors.ErrBinanceAPI,
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			"binance",
			nil,
		)
	}
	return body, nil
}

// doSignedRequest performs an authenticated request with HMAC-SHA256 signature.
func (c *BinanceClient) doSignedRequest(method, path string, params url.Values) ([]byte, error) {
	c.limiter.Wait()

	if params == nil {
		params = url.Values{}
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	queryString := params.Encode()
	signature := c.signer.Sign(queryString)
	queryString += "&signature=" + signature

	fullURL := c.baseURL + path + "?" + queryString

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrNetwork, "failed to create request", "binance", err)
	}
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrNetwork, "request failed", "binance", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrNetwork, "failed to read response", "binance", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewAppError(
			apperrors.ErrBinanceAPI,
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			"binance",
			nil,
		)
	}
	return body, nil
}
