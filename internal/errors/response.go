package errors

import (
        "net/http"

        "github.com/gofiber/fiber/v2"
)

// Response is the standard API response format
type Response struct {
        Code    int         `json:"code"`
        Message string      `json:"message"`
        Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(data interface{}) *Response {
        return &Response{
                Code:    CodeSuccess,
                Message: "success",
                Data:    data,
        }
}

// ErrorResponse creates an error response.
// Handles three error types:
//  1. BizError       → returns its code and message directly
//  2. fiber.Error     → maps HTTP status to business code (404→10005, 405→10005, etc.)
//  3. generic error   → returns CodeInternalError (10001)
func ErrorResponse(err error) *Response {
        // Priority 1: BizError (our own business error)
        if bizErr, ok := err.(*BizError); ok {
                return &Response{
                        Code:    bizErr.Code,
                        Message: bizErr.Message,
                }
        }

        // Priority 2: fiber.Error (framework-level error, e.g. 404 route not found)
        if fiberErr, ok := err.(*fiber.Error); ok {
                return &Response{
                        Code:    httpStatusToBizCode(fiberErr.Code),
                        Message: fiberErr.Message,
                }
        }

        // Priority 3: generic error (e.g. panic recovered, unexpected error)
        msg := err.Error()
        if msg == "" {
                msg = "internal server error"
        }
        return &Response{
                Code:    CodeInternalError,
                Message: msg,
        }
}

// httpStatusToBizCode maps HTTP status codes to business error codes.
// This ensures fiber.Error (e.g. 404 Not Found) returns meaningful business codes
// instead of the generic CodeInternalError (10001).
func httpStatusToBizCode(httpStatus int) int {
        switch httpStatus {
        case http.StatusNotFound:
                return CodeNotFound
        case http.StatusBadRequest:
                return CodeInvalidParams
        case http.StatusUnauthorized:
                return CodeUnauthorized
        case http.StatusForbidden:
                return CodeForbidden
        case http.StatusMethodNotAllowed:
                return CodeNotFound
        case http.StatusConflict:
                return CodeDuplicateRequest
        case http.StatusTooManyRequests:
                return CodeTooManyRequests
        case http.StatusServiceUnavailable:
                return CodeGameMaintenance
        case http.StatusGatewayTimeout:
                return CodeVendorTimeout
        default:
                return CodeInternalError
        }
}

// PagedData wraps paged results
type PagedData struct {
        List     interface{} `json:"list"`
        Total    int64       `json:"total"`
        Page     int         `json:"page"`
        PageSize int         `json:"page_size"`
        HasMore  bool        `json:"has_more"`
}
