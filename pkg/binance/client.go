package binance

import (
	"fmt"
	"io"
	apperrors "money-loves-me/internal/errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.binance.com"
	defaultWsURL   = "wss://stream.binance.com:9443"

	// 速率限制：每分钟 1200 个请求 = 每秒 20 个。
	rateLimitPerSec = 20
)

// BinanceClient 是与 Binance API 交互的主入口。
type BinanceClient struct {
	apiKey     string
	secretKey  string
	baseURL    string
	wsURL      string
	httpClient *http.Client
	signer     *HMACSigner
	limiter    *rateLimiter
}

// rateLimiter 实现了一个简单的令牌桶式速率限制器。
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

// Wait 阻塞直到有可用的令牌。
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

// Stop 关闭速率限制器的 goroutine。
func (rl *rateLimiter) Stop() {
	close(rl.stopCh)
}

// ClientOption 允许自定义 BinanceClient。
type ClientOption func(*BinanceClient)

// WithBaseURL 覆盖默认的 REST 基础 URL。
func WithBaseURL(u string) ClientOption {
	return func(c *BinanceClient) { c.baseURL = u }
}

// WithWsURL 覆盖默认的 WebSocket URL。
func WithWsURL(u string) ClientOption {
	return func(c *BinanceClient) { c.wsURL = u }
}

// WithHTTPClient 覆盖默认的 HTTP 客户端。
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *BinanceClient) { c.httpClient = hc }
}

// NewBinanceClient 使用给定的凭证创建一个新的 BinanceClient。
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

// Close 释放客户端持有的资源。
func (c *BinanceClient) Close() {
	c.limiter.Stop()
}

// doPublicRequest 执行一个未认证的 GET 请求。
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

// doSignedRequest 执行一个带有 HMAC-SHA256 签名的认证请求。
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
