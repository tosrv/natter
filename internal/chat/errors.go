// Package chat implements the messenger's business rules on top of the store.
package chat

import (
	"errors"
	"net/http"

	"github.com/tosrv/pmess/internal/store"
)

// Error is a client-facing error with a stable code and HTTP status.
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string { return e.Code }

var (
	ErrUserLimitReached     = &Error{"user_limit_reached", http.StatusForbidden}
	ErrDeveloperUnavailable = &Error{"developer_unavailable", http.StatusServiceUnavailable}
	ErrNotFound             = &Error{"not_found", http.StatusNotFound}
	ErrForbidden            = &Error{"forbidden", http.StatusForbidden}
	ErrMessageEmpty         = &Error{"message_empty", http.StatusBadRequest}
	ErrMessageTooLong       = &Error{"message_too_long", http.StatusBadRequest}
	ErrUsernameInvalid      = &Error{"username_invalid", http.StatusBadRequest}
	ErrUsernameTaken        = &Error{"username_taken", http.StatusConflict}
	ErrUsernameChangeLimit  = &Error{"username_change_limit", http.StatusConflict}
	ErrUsernameRequired     = &Error{"username_required", http.StatusForbidden}
	ErrDisplayNameInvalid   = &Error{"display_name_invalid", http.StatusBadRequest}
	ErrQueryTooShort        = &Error{"query_too_short", http.StatusBadRequest}
	ErrRateLimited          = &Error{"rate_limited", http.StatusTooManyRequests}
	ErrBadRequest           = &Error{"bad_request", http.StatusBadRequest}
	ErrCaptchaFailed        = &Error{"captcha_failed", http.StatusBadRequest}
	ErrCaptchaUnavailable   = &Error{"captcha_unavailable", http.StatusServiceUnavailable}
	ErrUnauthorized         = &Error{"unauthorized", http.StatusUnauthorized}
)

// mapErr translates store sentinels into chat errors; others pass through.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, store.ErrSlotsFull):
		return ErrUserLimitReached
	case errors.Is(err, store.ErrUsernameTaken):
		return ErrUsernameTaken
	case errors.Is(err, store.ErrUsernameChangeLimit):
		return ErrUsernameChangeLimit
	}
	return err
}
