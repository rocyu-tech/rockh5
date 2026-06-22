package errors_test

import (
	"net/http"
	"testing"

	"github.com/rocyu-tech/rockgame/internal/errors"
)

func TestPredefinedErrorsHaveHTTPCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *errors.BizError
		want int
	}{
		{"ErrSuccess", errors.ErrSuccess, 0},
		{"ErrInternal", errors.ErrInternal, 500},
		{"ErrInvalidParams", errors.ErrInvalidParams, 400},
		{"ErrUnauthorized", errors.ErrUnauthorized, 401},
		{"ErrForbidden", errors.ErrForbidden, 403},
		{"ErrNotFound", errors.ErrNotFound, 404},
		{"ErrTooManyRequests", errors.ErrTooManyRequests, 429},
		{"ErrInsufficientBalance", errors.ErrInsufficientBalance, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.HTTP != tt.want {
				t.Errorf("%s.HTTP = %d, want %d", tt.name, tt.err.HTTP, tt.want)
			}
			if tt.err.Code == 0 && tt.name != "ErrSuccess" {
				t.Errorf("%s.Code = 0, expected non-zero", tt.name)
			}
			if tt.err.Message == "" {
				t.Errorf("%s.Message is empty", tt.name)
			}
		})
	}
}

func TestNewBizError(t *testing.T) {
	err := errors.New(errors.CodeInvalidParams, "test message")
	if err.Code != errors.CodeInvalidParams {
		t.Errorf("Code = %d, want %d", err.Code, errors.CodeInvalidParams)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %q, want %q", err.Message, "test message")
	}
	if err.HTTP != 0 {
		t.Errorf("default HTTP = %d, want 0", err.HTTP)
	}
}

func TestWithHTTP(t *testing.T) {
	err := errors.New(errors.CodeInternalError, "internal").WithHTTP(http.StatusInternalServerError)
	if err.HTTP != 500 {
		t.Errorf("HTTP = %d, want 500", err.HTTP)
	}
	err2 := err.WithHTTP(502)
	if err != err2 {
		t.Error("WithHTTP should return same *BizError pointer")
	}
	if err2.HTTP != 502 {
		t.Errorf("HTTP after second WithHTTP = %d, want %d", err2.HTTP)
	}
}

func TestErrorMethod(t *testing.T) {
	err := errors.New(10001, "something went wrong")
	str := err.Error()
	if str == "" {
		t.Error("Error() returned empty string")
	}
	if len(str) < 10 {
		t.Errorf("Error() string too short: %q", str)
	}
}

func TestSuccessResponse(t *testing.T) {
	resp := errors.SuccessResponse(map[string]string{"key": "value"})
	if resp.Code != errors.CodeSuccess {
		t.Errorf("SuccessResponse.Code = %d, want %d", resp.Code, errors.CodeSuccess)
	}
	if resp.Data == nil {
		t.Error("SuccessResponse.Data is nil")
	}
}

func TestErrorResponse(t *testing.T) {
	bizErr := errors.New(errors.CodeInvalidParams, "bad input").WithHTTP(400)
	resp := errors.ErrorResponse(bizErr)
	if resp.Code != errors.CodeInvalidParams {
		t.Errorf("ErrorResponse.Code = %d, want %d", resp.Code, errors.CodeInvalidParams)
	}
	if resp.Message != "bad input" {
		t.Errorf("ErrorResponse.Message = %q, want %q", resp.Message, "bad input")
	}
}

func TestPagedData(t *testing.T) {
	pd := &errors.PagedData{
		List:     []string{"a", "b"},
		Total:    100,
		Page:     1,
		PageSize: 20,
		HasMore:  true,
	}
	if pd.Total != 100 {
		t.Errorf("Total = %d, want 100", pd.Total)
	}
	if !pd.HasMore {
		t.Error("HasMore should be true")
	}
}

func TestCodeFromError(t *testing.T) {
	bizErr := errors.New(errors.CodeNotFound, "not found").WithHTTP(404)
	code := errors.CodeFromError(bizErr)
	if code != errors.CodeNotFound {
		t.Errorf("CodeFromError = %d, want %d", code, errors.CodeNotFound)
	}
	code = errors.CodeFromError(errors.ErrInternal)
	if code != errors.CodeInternalError {
		t.Errorf("CodeFromError for generic error = %d, want %d", code, errors.CodeInternalError)
	}
}