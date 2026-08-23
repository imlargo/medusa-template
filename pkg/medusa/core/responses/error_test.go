package responses

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// render runs WriteError against a throwaway context and decodes the result.
func render(t *testing.T, err error, setup func(*gin.Context)) (int, ErrorResponse) {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if setup != nil {
		setup(c)
	}

	WriteError(c, err)

	var body ErrorResponse
	if rec.Body.Len() > 0 {
		if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
			t.Fatalf("response body is not an ErrorResponse: %v (%q)", decodeErr, rec.Body.String())
		}
	}
	return rec.Code, body
}

// The status is derived from the error itself, which is what replaced the
// per-code switch that used to live in the context package.
func TestWriteErrorDerivesStatusFromTheError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   ErrorCode
	}{
		{"bad request", BadRequest("bad"), http.StatusBadRequest, ErrCodeBadRequest},
		{"validation", Validation("invalid", nil), http.StatusBadRequest, ErrCodeValidation},
		{"unauthorized", Unauthorized("nope"), http.StatusUnauthorized, ErrCodeUnauthorized},
		{"forbidden", Forbidden("nope"), http.StatusForbidden, ErrCodeForbidden},
		{"not found", NotFound("user"), http.StatusNotFound, ErrCodeNotFound},
		{"conflict", Conflict("taken"), http.StatusConflict, ErrCodeConflict},
		{"too many requests", TooManyRequests(30), http.StatusTooManyRequests, ErrCodeTooManyRequests},
		{"unavailable", ServiceUnavailable("draining"), http.StatusServiceUnavailable, ErrCodeServiceUnavailable},
		{"internal", Internal(errors.New("boom")), http.StatusInternalServerError, ErrCodeInternalServer},
		{"unclassified", errors.New("boom"), http.StatusInternalServerError, ErrCodeInternalServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := render(t, tt.err, nil)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Status != tt.wantStatus {
				t.Errorf("body status = %d, want %d", body.Status, tt.wantStatus)
			}
		})
	}
}

// The internal cause is for the log, never for the client.
func TestWriteErrorNeverLeaksTheInternalCause(t *testing.T) {
	secret := "connection to 10.0.0.5 refused: password authentication failed"

	status, body := render(t, Internal(errors.New(secret)), nil)

	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
	}

	serialized, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	if body := string(serialized); strings.Contains(body, secret) {
		t.Errorf("response leaked the internal cause: %s", body)
	}
}

// The cause still has to reach the access log, which reads Gin's error list.
func TestWriteErrorRecordsTheCauseForLogging(t *testing.T) {
	cause := errors.New("boom")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	WriteError(c, Internal(cause))

	if len(c.Errors) == 0 {
		t.Fatal("c.Errors is empty, want the internal cause recorded")
	}
	if got := c.Errors.Last().Err; !errors.Is(got, cause) {
		t.Errorf("recorded error = %v, want %v", got, cause)
	}
}

func TestWriteErrorIncludesTheRequestID(t *testing.T) {
	const id = "req-123"

	_, body := render(t, NotFound("user"), func(c *gin.Context) {
		c.Set(RequestIDKey, id)
	})

	if body.RequestID != id {
		t.Errorf("request_id = %q, want %q", body.RequestID, id)
	}
}

func TestWriteErrorOnNilWritesNothing(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	WriteError(c, nil)

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

// A classified error keeps its classification through wrapping, which is what
// lets services wrap freely without turning a 404 into a 500.
func TestFromUnwrapsAWrappedError(t *testing.T) {
	original := NotFound("user")
	wrapped := fmt.Errorf("loading profile: %w", original)

	got := From(wrapped)

	if got.Status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got.Status, http.StatusNotFound)
	}
	if got.Code != ErrCodeNotFound {
		t.Errorf("code = %q, want %q", got.Code, ErrCodeNotFound)
	}
}

func TestFromNil(t *testing.T) {
	if got := From(nil); got != nil {
		t.Errorf("From(nil) = %v, want nil", got)
	}
}

func TestWrapKeepsClassificationAndChainsTheCause(t *testing.T) {
	original := Conflict("email already registered")

	wrapped := Wrap(original, "could not create the account")

	if wrapped.Status != http.StatusConflict {
		t.Errorf("status = %d, want %d", wrapped.Status, http.StatusConflict)
	}
	if wrapped.Message != "could not create the account" {
		t.Errorf("message = %q, want the new one", wrapped.Message)
	}
	if !errors.Is(wrapped, original) {
		t.Error("errors.Is(wrapped, original) = false, want the cause to be reachable")
	}
}

func TestWrapClassifiesAnUnknownErrorAsInternal(t *testing.T) {
	wrapped := Wrap(errors.New("boom"), "failed")

	if wrapped.Status != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", wrapped.Status, http.StatusInternalServerError)
	}
}

func TestErrorMessageIncludesTheCauseForLogs(t *testing.T) {
	err := InternalWithMessage("could not save", errors.New("disk full"))

	if got, want := err.Error(), "could not save: disk full"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAbortWithErrorStopsTheChain(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	AbortWithError(c, Unauthorized("nope"))

	if !c.IsAborted() {
		t.Error("IsAborted() = false, want the chain stopped")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
