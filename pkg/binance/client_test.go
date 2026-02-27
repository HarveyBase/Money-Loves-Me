package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "money-loves-me/internal/errors"

	"pgregory.net/rapid"
)

// Feature: binance-trading-system, Property 3: 无效凭证返回结构化错误
// Validates: Requirements 1.2
//
// Property 3: 对于任意无效的 API Key 或 Secret Key，认证请求应返回包含
// 错误码和描述的错误，且永远不应返回成功响应。

func TestProperty3_InvalidCredentialsReturnStructuredError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成任意无效的 API key 和 secret key。
		apiKey := rapid.String().Draw(t, "apiKey")
		secretKey := rapid.String().Draw(t, "secretKey")

		// 随机选择一个 Binance 在认证失败时会返回的 HTTP 状态码。
		authErrorCode := rapid.SampledFrom([]int{
			http.StatusUnauthorized, // 401
			http.StatusForbidden,    // 403
		}).Draw(t, "httpStatus")

		// 生成一个随机的 Binance 风格错误响应体。
		binanceErrCode := rapid.IntRange(-2000, -1000).Draw(t, "binanceErrCode")
		binanceErrMsg := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(t, "binanceErrMsg")

		errBody, _ := json.Marshal(map[string]interface{}{
			"code": binanceErrCode,
			"msg":  binanceErrMsg,
		})

		// 创建一个模拟服务器，对签名端点始终返回认证错误。
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(authErrorCode)
			w.Write(errBody)
		}))
		defer server.Close()

		client := NewBinanceClient(apiKey, secretKey, WithBaseURL(server.URL))
		defer client.Close()

		// 随机选择一个认证方法进行调用。
		type authMethod struct {
			name string
			call func() error
		}
		methods := []authMethod{
			{"GetAccountInfo", func() error { _, err := client.GetAccountInfo(); return err }},
			{"CreateOrder", func() error {
				_, err := client.CreateOrder(CreateOrderRequest{
					Symbol: "BTCUSDT",
					Side:   "BUY",
					Type:   "MARKET",
				})
				return err
			}},
			{"CancelOrder", func() error { _, err := client.CancelOrder("BTCUSDT", 12345); return err }},
		}

		method := methods[rapid.IntRange(0, len(methods)-1).Draw(t, "methodIndex")]

		// 执行认证请求。
		err := method.call()

		// --- 子属性 1：错误不能为 nil（永远不返回成功） ---
		if err == nil {
			t.Fatalf("method %s returned nil error for invalid credentials (apiKey=%q, secretKey=%q)",
				method.name, apiKey, secretKey)
		}

		// --- 子属性 2：错误必须是结构化的 AppError ---
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("method %s returned non-AppError type %T: %v", method.name, err, err)
		}

		// --- 子属性 3：AppError 必须包含有效的错误码 ---
		if appErr.Code == 0 {
			t.Fatalf("method %s returned AppError with zero Code", method.name)
		}

		// --- 子属性 4：AppError 必须包含非空的消息 ---
		if appErr.Message == "" {
			t.Fatalf("method %s returned AppError with empty Message", method.name)
		}

		// --- 子属性 5：错误消息应包含 HTTP 状态码 ---
		expectedFragment := fmt.Sprintf("HTTP %d", authErrorCode)
		if len(appErr.Message) < len(expectedFragment) {
			t.Fatalf("method %s: AppError message %q is too short to contain status info", method.name, appErr.Message)
		}
	})
}
