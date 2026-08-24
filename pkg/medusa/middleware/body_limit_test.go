package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"github.com/imlargo/medusa/pkg/medusa/middleware"
)

func init() { gin.SetMode(gin.TestMode) }

type payload struct {
	Text string `json:"text" binding:"required"`
}

func post(t *testing.T, limit int64, body string) (int, responses.ErrorResponse) {
	t.Helper()

	router := gin.New()
	router.Use(middleware.NewBodyLimitMiddleware(limit))
	router.POST("/x", medusa.Handle(func(ctx *medusa.Context, in *payload) (*payload, error) {
		return in, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var decoded responses.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec.Code, decoded
}

func TestBodyUnderTheLimitIsAccepted(t *testing.T) {
	if status, _ := post(t, 1024, `{"text":"hello"}`); status != http.StatusOK {
		t.Errorf("status = %d, want %d", status, http.StatusOK)
	}
}

// An oversized body is not a validation problem: reporting it as a 400 sends the
// client hunting for a field that is perfectly fine.
func TestOversizedBodyIs413NotAValidationError(t *testing.T) {
	oversized := `{"text":"` + strings.Repeat("a", 2048) + `"}`

	status, body := post(t, 512, oversized)

	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", status, http.StatusRequestEntityTooLarge)
	}
	if body.Code != responses.ErrCodePayloadTooLarge {
		t.Errorf("code = %q, want %q", body.Code, responses.ErrCodePayloadTooLarge)
	}

	details, ok := body.Details.(map[string]any)
	if !ok {
		t.Fatalf("details = %v, want the limit reported", body.Details)
	}
	if details["max_bytes"] != float64(512) {
		t.Errorf("details[max_bytes] = %v, want 512", details["max_bytes"])
	}
}
