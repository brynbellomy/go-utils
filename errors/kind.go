package errors

// Kind classifies an error. A Kind is itself an error, usable as a sentinel
// that can be returned, wrapped, and matched with errors.Is, and it is an
// Option, so passing it to New, Wrap, or Set classifies the resulting error,
// which then also matches the Kind under errors.Is.
//
// Each Kind carries a default Fault, Retryability, and HTTP status code.
// GetFault, GetRetryability, and GetStatusCode fall back to them for any
// layer of a chain that sets a Kind but not the property itself.
type Kind uint8

const (
	KindUnknown Kind = iota
	ErrBadRequest
	ErrUnauthorized
	ErrForbidden
	ErrNotFound
	ErrConflict
	ErrTooManyRequests
	ErrUnimplemented
	ErrInternalServerError
	ErrServiceUnavailable
	ErrTimeout
	kindCount
)

var _ Option = KindUnknown

type kindInfo struct {
	name   string
	msg    string
	status StatusCode
	fault  Fault
	retry  Retryability
}

var kinds = [kindCount]kindInfo{
	KindUnknown:            {"unknown", "unknown error", 0, FaultUnknown, UnknownRetryability},
	ErrBadRequest:          {"bad_request", "bad request", 400, FaultCaller, UnknownRetryability},
	ErrUnauthorized:        {"unauthorized", "unauthorized", 401, FaultCaller, UnknownRetryability},
	ErrForbidden:           {"forbidden", "forbidden", 403, FaultCaller, UnknownRetryability},
	ErrNotFound:            {"not_found", "not found", 404, FaultCaller, UnknownRetryability},
	ErrConflict:            {"conflict", "conflict", 409, FaultCaller, UnknownRetryability},
	ErrTooManyRequests:     {"too_many_requests", "too many requests", 429, FaultCaller, Retryable},
	ErrUnimplemented:       {"unimplemented", "not implemented", 501, FaultCaller, UnknownRetryability},
	ErrInternalServerError: {"internal", "internal server error", 500, FaultInternal, UnknownRetryability},
	ErrServiceUnavailable:  {"unavailable", "service unavailable", 503, FaultInternal, Retryable},
	ErrTimeout:             {"timeout", "timeout", 504, FaultInternal, Retryable},
}

func (k Kind) info() kindInfo {
	if k >= kindCount {
		return kinds[KindUnknown]
	}
	return kinds[k]
}

// Error returns the Kind's message, such as "not found".
func (k Kind) Error() string { return k.info().msg }

// String returns a lowercase, stable, space-free name for the Kind, such as
// "not_found", suitable for log fields and metrics labels. Out-of-range
// values return "unknown".
func (k Kind) String() string { return k.info().name }

// StatusCode returns the Kind's default HTTP status code, or 0 for
// KindUnknown.
func (k Kind) StatusCode() int { return int(k.info().status) }

// Fault returns the Kind's default Fault.
func (k Kind) Fault() Fault { return k.info().fault }

// Retryability returns the Kind's default Retryability.
func (k Kind) Retryability() Retryability { return k.info().retry }

func (k Kind) applyTo(e *Error) { e.kind = k }

// GetKind returns the Kind of the outermost layer of err's chain that sets
// one, whether as an option on an *Error or as a Kind value in the chain.
// Searches an *Error's wrapped error before its cause and a join's children
// first to last. Returns KindUnknown if no layer sets a Kind.
func GetKind(err error) Kind {
	k, _ := find(err, kindOf)
	return k
}

// kindOf returns the Kind err itself sets, not counting its chain.
func kindOf(err error) (Kind, bool) {
	switch e := err.(type) {
	case *Error:
		return e.kind, e.kind != KindUnknown
	case Kind:
		return e, e != KindUnknown
	default:
		return KindUnknown, false
	}
}
