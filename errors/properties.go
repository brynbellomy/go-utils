package errors

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type Fault uint8

const (
	FaultUnknown Fault = iota
	FaultCaller
	FaultInternal
)

type Retryability uint8

const (
	UnknownRetryability Retryability = iota
	Retryable
	NonRetryable
)

type StatusCode int

var (
	ErrBadRequest          = WithNew("bad request").Set(StatusCode(http.StatusBadRequest), FaultCaller)
	ErrUnauthorized        = WithNew("unauthorized").Set(StatusCode(http.StatusUnauthorized), FaultCaller)
	ErrForbidden           = WithNew("forbidden").Set(StatusCode(http.StatusForbidden), FaultCaller)
	ErrNotFound            = WithNew("not found").Set(StatusCode(http.StatusNotFound), FaultCaller)
	ErrTooManyRequests     = WithNew("too many requests").Set(StatusCode(http.StatusTooManyRequests), FaultCaller)
	ErrUnimplemented       = WithNew("not implemented").Set(StatusCode(http.StatusNotImplemented), FaultCaller)
	ErrInternalServerError = WithNew("internal server error").Set(StatusCode(http.StatusInternalServerError), FaultInternal)
	ErrServiceUnavailable  = WithNew("service unavailable").Set(StatusCode(http.StatusServiceUnavailable), FaultInternal)
)

// Fields represents structured key-value pairs for logging. Fields are formatted
// as logfmt when the error is printed: "error message key1=value1 key2="quoted value"".
// Use %s or %v to include fields in output; %q outputs only the error message.
type Fields []any

// Add appends additional key-value pairs to the Fields slice.
// Can be called incrementally to build up context as it becomes available.
func (f *Fields) Add(fields ...any) {
	if f == nil {
		arr := Fields(make([]any, 0, len(fields)))
		f = &arr
	}
	*f = append(*f, fields...)
}

func (f Fields) List() []any {
	return f
}

// withMetadata is a unified error wrapper that combines properties (Fault, StatusCode,
// Retryability) and fields (key-value pairs for logging) in a single allocation.
type withMetadata struct {
	parent       error
	fault        Fault
	statusCode   StatusCode
	retryability Retryability
	fields       Fields
}

// WithMetadata wraps an error with properties and/or fields.
// Accepts Fault, StatusCode, Retryability, Fields, and individual field values.
func WithMetadata(err error, items ...any) error {
	if err == nil {
		return nil
	}

	wm := &withMetadata{
		parent: err,
	}

	var pendingFields []any

	for _, item := range items {
		switch v := item.(type) {
		case Fault:
			wm.fault = v
		case StatusCode:
			wm.statusCode = v
		case Retryability:
			wm.retryability = v
		case Fields:
			pendingFields = append(pendingFields, v...)
		default:
			// Treat as individual field key or value
			pendingFields = append(pendingFields, v)
		}
	}

	if len(pendingFields) > 0 {
		wm.fields = pendingFields
	}

	return wm
}

func (wm *withMetadata) Error() string {
	if wm.parent != nil {
		return wm.parent.Error()
	}
	return "error with metadata"
}

func (wm *withMetadata) Unwrap() error {
	return wm.parent
}

func (wm *withMetadata) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// Verbose format: show parent error with stack if available
			if wm.parent != nil {
				fmt.Fprintf(s, "%+v", wm.parent)
			} else {
				io.WriteString(s, "error with metadata")
			}
		} else {
			// Standard %v format
			io.WriteString(s, wm.Error())
		}
		// Add all fields in logfmt format
		allFields := GetFields(wm)
		if len(allFields) > 0 {
			io.WriteString(s, " ")
			formatLogfmtFields(s, allFields)
		}
	case 's':
		io.WriteString(s, wm.Error())
		// Add all fields in logfmt format
		allFields := GetFields(wm)
		if len(allFields) > 0 {
			io.WriteString(s, " ")
			formatLogfmtFields(s, allFields)
		}
	case 'q':
		// For quoted format, just quote the error message without fields
		fmt.Fprintf(s, "%q", wm.Error())
	}
}

type unwrapper interface {
	Unwrap() error
}

// IsRetryable traverses the error chain looking for a Retryable marker.
// Returns true only if Retryable is explicitly set somewhere in the chain.
func IsRetryable(err error) bool {
	for err != nil {
		if wm, ok := err.(*withMetadata); ok {
			if wm.retryability == Retryable {
				return true
			}
			if wm.retryability == NonRetryable {
				return false
			}
			err = wm.parent
		} else {
			// Try unwrapping via standard Unwrap() method
			unwrapper, ok := err.(unwrapper)
			if !ok {
				break
			}
			err = unwrapper.Unwrap()
		}
	}
	return false
}

// GetStatusCode traverses the error chain and returns the first non-zero status code found.
// Outer layers override inner layers when explicitly set.
// Supports errors.Join by checking multiple unwrapped errors.
func GetStatusCode(err error) int {
	for err != nil {
		if wm, ok := err.(*withMetadata); ok {
			if wm.statusCode != 0 {
				return int(wm.statusCode)
			}
			err = wm.parent
		} else {
			// Try unwrapping via multi-error Unwrap() []error (e.g., errors.Join)
			if multiUnwrapper, ok := err.(interface{ Unwrap() []error }); ok {
				for _, e := range multiUnwrapper.Unwrap() {
					if code := GetStatusCode(e); code != 0 {
						return code
					}
				}
				return 0
			}

			// Try unwrapping via standard Unwrap() error
			unwrapper, ok := err.(unwrapper)
			if !ok {
				break
			}
			err = unwrapper.Unwrap()
		}
	}
	return 0
}

// GetFault traverses the error chain and returns the first non-unknown fault found.
// Outer layers override inner layers when explicitly set.
func GetFault(err error) Fault {
	for err != nil {
		if wm, ok := err.(*withMetadata); ok {
			if wm.fault != FaultUnknown {
				return wm.fault
			}
			err = wm.parent
		} else {
			// Try unwrapping via standard Unwrap() method
			unwrapper, ok := err.(unwrapper)
			if !ok {
				break
			}
			err = unwrapper.Unwrap()
		}
	}
	return FaultUnknown
}

// GetFields extracts all fields from an error chain.
// It traverses the error chain and collects fields from all withMetadata wrappers.
func GetFields(err error) Fields {
	var fields []any
	for err != nil {
		wm := &withMetadata{}
		if errors.As(err, &wm) {
			if len(wm.fields) > 0 {
				fields = append(fields, wm.fields...)
			}
			err = wm.parent
		} else {
			break
		}
	}
	return fields
}

// ListFields is an alias for GetFields.
func ListFields(err error) []any {
	return GetFields(err)
}

func formatLogfmtFields(w io.Writer, fields []any) {
	for i := 0; i < len(fields); i += 2 {
		if i > 0 {
			io.WriteString(w, " ")
		}

		// Write key
		key := fmt.Sprint(fields[i])
		io.WriteString(w, key)
		io.WriteString(w, "=")

		// Write value
		if i+1 < len(fields) {
			value := fields[i+1]
			switch v := value.(type) {
			case string:
				// Quote strings that contain spaces or special characters
				if needsQuoting(v) {
					fmt.Fprintf(w, "%q", v)
				} else {
					io.WriteString(w, v)
				}
			default:
				fmt.Fprint(w, value)
			}
		}
	}
}

func needsQuoting(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '=' || r == '"' || r == '\n' || r == '\t' || r == '\r' {
			return true
		}
	}
	return false
}
