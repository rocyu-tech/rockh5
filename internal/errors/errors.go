package errors

import (
        "fmt"
        "net/http"
)

// Business error codes
const (
        CodeSuccess            = 0
        CodeInternalError      = 10001
        CodeInvalidParams      = 10002
        CodeUnauthorized       = 10003
        CodeForbidden          = 10004
        CodeNotFound           = 10005
        CodeTooManyRequests    = 10006
        CodeUserNotFound       = 20001
        CodeInvalidPassword    = 20002
        CodeUserExists         = 20003
        CodeAccountDisabled    = 20004
        CodeTokenExpired       = 20005
        CodeInvalidToken       = 20006
        CodeInsufficientBalance = 30001
        CodeOrderNotFound      = 30002
        CodeOrderExpired       = 30003
        CodePaymentFailed      = 30004
        CodeDuplicateRequest   = 30005
        CodeGameNotFound       = 40001
        CodeGameMaintenance    = 40002
        CodeVendorTimeout      = 40003
        CodeActivityExpired      = 50001
        CodeActivityStockEmpty   = 50002
        CodeAlreadyCheckedIn     = 50003
        CodeActivityNotStarted   = 50004
        CodeActivityEnded        = 50005
        CodeCheckInRewardFailed  = 50006
        CodeWheelSpinsExhausted  = 50007
        CodeWheelCooldown        = 50008
        CodeWheelNotActive       = 50009
        CodeWheelPrizeStockEmpty = 50010
        CodeWheelRewardFailed    = 50011
        CodeKYCPending         = 60001
        CodeKYCRejected        = 60002

        // Spin wheel (C++ port) error codes 70001–70099
        CodeSpinDayLimit       = 70001
        CodeSpinNoChance       = 70002
        CodeSpinAmountFull     = 70003
        CodeSpinBindPhone      = 70004
        CodeSpinUserDataErr    = 70005
        CodeSpinNotActive      = 70006
        CodeSpinOrderNotFound  = 70007
        CodeSpinAlreadyPending = 70008
)

// BizError represents a business error with code and message
type BizError struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
        HTTP    int    `json:"-"` // HTTP status code
}

func (e *BizError) Error() string {
        return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// New creates a new BizError
func New(code int, message string) *BizError {
        return &BizError{Code: code, Message: message}
}

// WithHTTP sets the HTTP status code
func (e *BizError) WithHTTP(status int) *BizError {
        e.HTTP = status
        return e
}

// Predefined errors
var (
        ErrSuccess             = New(CodeSuccess, "success")
        ErrInternal            = New(CodeInternalError, "internal server error").WithHTTP(http.StatusInternalServerError)
        ErrInvalidParams       = New(CodeInvalidParams, "invalid parameters").WithHTTP(http.StatusBadRequest)
        ErrUnauthorized        = New(CodeUnauthorized, "unauthorized").WithHTTP(http.StatusUnauthorized)
        ErrForbidden           = New(CodeForbidden, "forbidden").WithHTTP(http.StatusForbidden)
        ErrNotFound            = New(CodeNotFound, "resource not found").WithHTTP(http.StatusNotFound)
        ErrTooManyRequests     = New(CodeTooManyRequests, "too many requests").WithHTTP(http.StatusTooManyRequests)
        ErrUserNotFound        = New(CodeUserNotFound, "user not found").WithHTTP(http.StatusNotFound)
        ErrInvalidPassword     = New(CodeInvalidPassword, "invalid password").WithHTTP(http.StatusUnauthorized)
        ErrUserExists          = New(CodeUserExists, "user already exists").WithHTTP(http.StatusConflict)
        ErrAccountDisabled     = New(CodeAccountDisabled, "account is disabled").WithHTTP(http.StatusForbidden)
        ErrTokenExpired        = New(CodeTokenExpired, "token expired").WithHTTP(http.StatusUnauthorized)
        ErrInvalidToken        = New(CodeInvalidToken, "invalid token").WithHTTP(http.StatusUnauthorized)
        ErrInsufficientBalance = New(CodeInsufficientBalance, "insufficient balance").WithHTTP(http.StatusPaymentRequired)
        ErrDuplicateRequest    = New(CodeDuplicateRequest, "duplicate request").WithHTTP(http.StatusConflict)
        ErrGameMaintenance     = New(CodeGameMaintenance, "game is under maintenance").WithHTTP(http.StatusServiceUnavailable)
        ErrVendorTimeout       = New(CodeVendorTimeout, "vendor request timeout").WithHTTP(http.StatusGatewayTimeout)
        ErrNotImplemented       = New(CodeInternalError, "not implemented").WithHTTP(http.StatusNotImplemented)
        ErrActivityExpired     = New(CodeActivityExpired, "activity has expired").WithHTTP(http.StatusBadRequest)
        ErrActivityStockEmpty  = New(CodeActivityStockEmpty, "activity stock is empty").WithHTTP(http.StatusConflict)
        ErrAlreadyCheckedIn    = New(CodeAlreadyCheckedIn, "already checked in today").WithHTTP(http.StatusConflict)
        ErrActivityNotStarted  = New(CodeActivityNotStarted, "activity has not started yet").WithHTTP(http.StatusBadRequest)
        ErrActivityEnded       = New(CodeActivityEnded, "activity has ended").WithHTTP(http.StatusBadRequest)
        ErrCheckInRewardFailed = New(CodeCheckInRewardFailed, "check-in reward failed").WithHTTP(http.StatusInternalServerError)
        ErrWheelSpinsExhausted = New(CodeWheelSpinsExhausted, "no spins remaining today").WithHTTP(http.StatusBadRequest)
        ErrWheelCooldown       = New(CodeWheelCooldown, "spin cooldown active, please wait").WithHTTP(http.StatusTooManyRequests)
        ErrWheelNotActive      = New(CodeWheelNotActive, "no active wheel activity").WithHTTP(http.StatusNotFound)
        ErrWheelPrizeStockEmpty = New(CodeWheelPrizeStockEmpty, "prize is out of stock").WithHTTP(http.StatusConflict)
        ErrWheelRewardFailed   = New(CodeWheelRewardFailed, "wheel reward failed").WithHTTP(http.StatusInternalServerError)

        // Spin wheel (C++ port) errors
        ErrSpinDayLimit       = New(CodeSpinDayLimit, "daily spin limit reached").WithHTTP(http.StatusBadRequest)
        ErrSpinNoChance       = New(CodeSpinNoChance, "no spin chances remaining").WithHTTP(http.StatusBadRequest)
        ErrSpinAmountFull     = New(CodeSpinAmountFull, "wheel amount already full").WithHTTP(http.StatusBadRequest)
        ErrSpinBindPhone      = New(CodeSpinBindPhone, "please bind phone number first").WithHTTP(http.StatusBadRequest)
        ErrSpinUserDataErr    = New(CodeSpinUserDataErr, "spin user data error").WithHTTP(http.StatusBadRequest)
        ErrSpinNotActive      = New(CodeSpinNotActive, "no active spin config found").WithHTTP(http.StatusNotFound)
        ErrSpinOrderNotFound  = New(CodeSpinOrderNotFound, "spin order not found").WithHTTP(http.StatusNotFound)
        ErrSpinAlreadyPending = New(CodeSpinAlreadyPending, "spin order already pending").WithHTTP(http.StatusConflict)
)

// Is checks if error is a BizError with specific code
func Is(err error, code int) bool {
        if bizErr, ok := err.(*BizError); ok {
                return bizErr.Code == code
        }
        return false
}

// CodeFromError extracts biz code from error
func CodeFromError(err error) int {
        if bizErr, ok := err.(*BizError); ok {
                return bizErr.Code
        }
        return CodeInternalError
}
