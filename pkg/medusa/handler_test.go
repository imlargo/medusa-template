package medusa_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

func init() { gin.SetMode(gin.TestMode) }

type greeting struct {
	Name string `json:"name" binding:"required"`
}

type greeted struct {
	Message string `json:"message"`
}

// call routes one request through handler and returns the status and body.
func call(t *testing.T, method, path, body string, handler gin.HandlerFunc) (int, map[string]any) {
	t.Helper()

	router := gin.New()
	router.Handle(method, "/x", handler)

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	decoded := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
		}
	}
	return rec.Code, decoded
}

func TestHandleRepliesOK(t *testing.T) {
	status, body := call(t, http.MethodPost, "/x", `{"name":"ada"}`,
		medusa.Handle(func(ctx *medusa.Context, in *greeting) (*greeted, error) {
			return &greeted{Message: "hello " + in.Name}, nil
		}))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if body["success"] != true {
		t.Errorf("success = %v, want true", body["success"])
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %v, want an object", body["data"])
	}
	if data["message"] != "hello ada" {
		t.Errorf("message = %v, want %q", data["message"], "hello ada")
	}
}

func TestHandleCreateReplies201(t *testing.T) {
	status, _ := call(t, http.MethodPost, "/x", `{"name":"ada"}`,
		medusa.HandleCreate(func(ctx *medusa.Context, in *greeting) (*greeted, error) {
			return &greeted{Message: "created"}, nil
		}))

	if status != http.StatusCreated {
		t.Errorf("status = %d, want %d", status, http.StatusCreated)
	}
}

// A missing required field must never reach the handler.
func TestHandleRejectsAnInvalidBodyBeforeTheHandlerRuns(t *testing.T) {
	called := false

	status, body := call(t, http.MethodPost, "/x", `{}`,
		medusa.Handle(func(ctx *medusa.Context, in *greeting) (*greeted, error) {
			called = true
			return nil, nil
		}))

	if called {
		t.Error("handler ran, want validation to reject the request first")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if body["code"] != string(responses.ErrCodeValidation) {
		t.Errorf("code = %v, want %q", body["code"], responses.ErrCodeValidation)
	}

	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("details = %v, want a field map", body["details"])
	}
	if details["name"] != "this field is required" {
		t.Errorf("details[name] = %v, want the required message", details["name"])
	}
}

// This is the payoff of returning errors: the handler says what went wrong and
// the status follows from that, with no response code at the call site.
func TestHandleDerivesTheStatusFromTheReturnedError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", responses.NotFound("user"), http.StatusNotFound},
		{"conflict", responses.Conflict("taken"), http.StatusConflict},
		{"unauthorized", responses.Unauthorized("nope"), http.StatusUnauthorized},
		{"unclassified becomes 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := call(t, http.MethodPost, "/x", `{"name":"ada"}`,
				medusa.Handle(func(ctx *medusa.Context, in *greeting) (*greeted, error) {
					return nil, tt.err
				}))

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestHandleGetNeedsNoBody(t *testing.T) {
	status, body := call(t, http.MethodGet, "/x", "",
		medusa.HandleGet(func(ctx *medusa.Context) (*greeted, error) {
			return &greeted{Message: "ok"}, nil
		}))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if body["success"] != true {
		t.Errorf("success = %v, want true", body["success"])
	}
}

func TestHandleDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		status, body := call(t, http.MethodDelete, "/x", "",
			medusa.HandleDelete(func(ctx *medusa.Context) error { return nil }))

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if body["message"] != responses.MessageDeleted {
			t.Errorf("message = %v, want %q", body["message"], responses.MessageDeleted)
		}
	})

	t.Run("failure", func(t *testing.T) {
		status, _ := call(t, http.MethodDelete, "/x", "",
			medusa.HandleDelete(func(ctx *medusa.Context) error {
				return responses.NotFound("user")
			}))

		if status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", status, http.StatusNotFound)
		}
	})
}

// Handler is for endpoints that write their own response; on success nothing
// should be appended to what the handler already sent.
func TestHandlerLeavesASuccessfulResponseAlone(t *testing.T) {
	status, body := call(t, http.MethodGet, "/x", "",
		medusa.Handler(func(ctx *medusa.Context) error {
			ctx.JSON(http.StatusTeapot, map[string]string{"custom": "yes"})
			return nil
		}))

	if status != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", status, http.StatusTeapot)
	}
	if body["custom"] != "yes" {
		t.Errorf("body = %v, want the handler's own payload", body)
	}
}
