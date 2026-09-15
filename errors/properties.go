package errors

import (
	"fmt"
	"io"
	"net/http"
)

type Fault uint8

const (
	FaultUnknown Fault = iota
	// FaultCaller indicates the client/caller is responsible for the error.
	FaultCaller
	// FaultInternal indicates the system itself failed (a bug, a resource
	// exhaustion, or any failure not attributable to a caller or a dependency).
	FaultInternal
	// FaultUpstream indicates a dependency the system relies on (an RPC node,
	// a database, a third-party API) failed.
	FaultUpstream
)

// String returns a lowercase, stable, space-free name for the Fault, suitable
// for use verbatim as a log field or Prometheus label value. Unrecognized
// values return "unknown".
func (f Fault) String() string {
	switch f {
	case FaultCaller:
		return "caller"
	case FaultInternal:
		return "internal"
	case FaultUpstream:
		return "upstream"
	default:
		return "unknown"
	}
}

type Retryability uint8

const (
	UnknownRetryability Retryability = iota
	Retryable
	NonRetryable
)

// String returns a lowercase, stable, space-free name for the Retryability,
// suitable for use verbatim as a log field or Prometheus label value.
// Unrecognized values return "unknown".
func (r Retryability) String() string {
	switch r {
	case Retryable:
		return "retryable"
	case NonRetryable:
		return "non_retryable"
	default:
		return "unknown"
	}
}

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

// Lookup returns the value associated with the first occurrence of key among
// the Fields' key/value pairs, comparing only string keys. Returns (nil, false)
// if key is not found. A dangling trailing key (an odd-length Fields with no
// paired value) is ignored rather than causing a panic.
func (f Fields) Lookup(key string) (any, bool) {
	for i := 0; i+1 < len(f); i += 2 {
		if k, ok := f[i].(string); ok && k == key {
			return f[i+1], true
		}
	}
	return nil, false
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
// Returns nil if err is nil or typed-nil.
func WithMetadata(err error, items ...any) error {
	if isNilError(err) {
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
	if wm == nil {
		return nilErrorString
	}
	if !isNilError(wm.parent) {
		return wm.parent.Error()
	}
	return "error with metadata"
}

func (wm *withMetadata) Unwrap() error {
	if wm == nil {
		return nil
	}
	return normalizeError(wm.parent)
}

func (wm *withMetadata) Format(s fmt.State, verb rune) {
	if wm == nil {
		io.WriteString(s, nilErrorString)
		return
	}

	switch verb {
	case 'v':
		if s.Flag('+') {
			// Verbose format: show parent error with stack if available
			if !isNilError(wm.parent) {
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

// multiUnwrapper is implemented by errors that join more than one error into
// a tree, such as the value returned by the re-exported Join (stdlib
// errors.Join).
type multiUnwrapper interface {
	Unwrap() []error
}

// walkProperty traverses err's chain outermost first, descending into a
// multi-unwrap node's children in order (so errors.Join is supported the
// same way GetStatusCode has always supported it). At each *withMetadata
// layer, extract reports whether that layer sets the property being sought;
// the first layer (searched outer to inner, and for a join, first child to
// last) for which extract reports true wins, so an outer wrapper around a
// join still overrides values found inside it. Returns the zero value of T
// and false if no layer in the chain sets the property.
func walkProperty[T any](err error, extract func(*withMetadata) (T, bool)) (T, bool) {
	var zero T
	for !isNilError(err) {
		if wm, ok := err.(*withMetadata); ok {
			if v, ok := extract(wm); ok {
				return v, true
			}
			err = normalizeError(wm.parent)
		} else if multi, ok := err.(multiUnwrapper); ok {
			for _, child := range multi.Unwrap() {
				if v, ok := walkProperty(child, extract); ok {
					return v, true
				}
			}
			return zero, false
		} else if u, ok := err.(unwrapper); ok {
			err = normalizeError(u.Unwrap())
		} else {
			break
		}
	}
	return zero, false
}

// GetRetryability traverses the error chain looking for an explicitly set
// Retryability value. It searches outermost first, and the first explicitly
// set value (Retryable or NonRetryable) wins. Descends into multi-unwrap
// values (e.g. errors.Join), checking each child in order. Returns
// UnknownRetryability if no layer in the chain sets one.
func GetRetryability(err error) Retryability {
	v, ok := walkProperty(err, func(wm *withMetadata) (Retryability, bool) {
		if wm.retryability == Retryable || wm.retryability == NonRetryable {
			return wm.retryability, true
		}
		return UnknownRetryability, false
	})
	if !ok {
		return UnknownRetryability
	}
	return v
}

// IsRetryable traverses the error chain looking for a Retryable marker.
// Returns true only if Retryable is explicitly set somewhere in the chain.
func IsRetryable(err error) bool {
	return GetRetryability(err) == Retryable
}

// GetStatusCode traverses the error chain and returns the first non-zero status code found.
// Outer layers override inner layers when explicitly set.
// Supports errors.Join by checking multiple unwrapped errors.
func GetStatusCode(err error) int {
	v, ok := walkProperty(err, func(wm *withMetadata) (int, bool) {
		if wm.statusCode != 0 {
			return int(wm.statusCode), true
		}
		return 0, false
	})
	if !ok {
		return 0
	}
	return v
}

// GetFault traverses the error chain and returns the first non-unknown fault found.
// Outer layers override inner layers when explicitly set. Descends into
// multi-unwrap values (e.g. errors.Join), checking each child in order.
func GetFault(err error) Fault {
	v, ok := walkProperty(err, func(wm *withMetadata) (Fault, bool) {
		if wm.fault != FaultUnknown {
			return wm.fault, true
		}
		return FaultUnknown, false
	})
	if !ok {
		return FaultUnknown
	}
	return v
}

// GetFields extracts all fields from an error chain, outermost layer first.
// At a multi-unwrap node (e.g. errors.Join), it collects from each child in
// order (each child walked outer to inner) and concatenates the results;
// fields are never deduplicated, since Fields.Lookup already returns the
// first (outermost/earliest) match for a given key.
func GetFields(err error) Fields {
	var fields Fields
	for !isNilError(err) {
		if wm, ok := err.(*withMetadata); ok {
			if len(wm.fields) > 0 {
				fields = append(fields, wm.fields...)
			}
			err = normalizeError(wm.parent)
		} else if multi, ok := err.(multiUnwrapper); ok {
			for _, child := range multi.Unwrap() {
				fields = append(fields, GetFields(child)...)
			}
			return fields
		} else if u, ok := err.(unwrapper); ok {
			err = normalizeError(u.Unwrap())
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

// LookupField returns the value for the first occurrence of key among the
// fields collected from err's chain via GetFields (outermost layer first, so
// an outer layer's field overrides an inner layer's field of the same key).
// Only string keys are compared. Returns (nil, false) if err is nil, typed-nil,
// or key is not present in any layer's fields.
func LookupField(err error, key string) (any, bool) {
	return GetFields(err).Lookup(key)
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
