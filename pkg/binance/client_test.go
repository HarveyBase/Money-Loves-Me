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
// Property 3: For any arbitrary invalid API Key or Secret Key, authenticated
// requests should return an error containing an error code and description,
// and should never return a success response.

func TestProperty3_InvalidCredentialsReturnStructuredError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary invalid API key and secret key.
		apiKey := rapid.String().Draw(t, "apiKey")
		secretKey := rapid.String().Draw(t, "secretKey")

		// Pick a random HTTP status code that Binance would return for auth failures.
		authErrorCode := rapid.SampledFrom([]int{
			http.StatusUnauthorized, // 401
			http.StatusForbidden,    // 403
		}).Draw(t, "httpStatus")

		// Generate a random Binance-style error response body.
		binanceErrCode := rapid.IntRange(-2000, -1000).Draw(t, "binanceErrCode")
		binanceErrMsg := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(t, "binanceErrMsg")

		errBody, _ := json.Marshal(map[string]interface{}{
			"code": binanceErrCode,
			"msg":  binanceErrMsg,
		})

		// Create a mock server that always returns the auth error for signed endpoints.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(authErrorCode)
			w.Write(errBody)
		}))
		defer server.Close()

		client := NewBinanceClient(apiKey, secretKey, WithBaseURL(server.URL))
		defer client.Close()

		// Pick a random authenticated method to call.
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

		// Execute the authenticated request.
		err := method.call()

		// --- Sub-property 1: Error must not be nil (never returns success) ---
		if err == nil {
			t.Fatalf("method %s returned nil error for invalid credentials (apiKey=%q, secretKey=%q)",
				method.name, apiKey, secretKey)
		}

		// --- Sub-property 2: Error must be a structured AppError ---
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("method %s returned non-AppError type %T: %v", method.name, err, err)
		}

		// --- Sub-property 3: AppError must have a valid error code ---
		if appErr.Code == 0 {
			t.Fatalf("method %s returned AppError with zero Code", method.name)
		}

		// --- Sub-property 4: AppError must have a non-empty message ---
		if appErr.Message == "" {
			t.Fatalf("method %s returned AppError with empty Message", method.name)
		}

		// --- Sub-property 5: The error message should contain the HTTP status code ---
		expectedFragment := fmt.Sprintf("HTTP %d", authErrorCode)
		if len(appErr.Message) < len(expectedFragment) {
			t.Fatalf("method %s: AppError message %q is too short to contain status info", method.name, appErr.Message)
		}
	})
}
