package errors

import (
	"fmt"
	"io"
	"log/slog"
)

// Option is a typed item accepted by New, Wrap, WithMetadata, and
// (*Error).Set that sets a property or appends fields. Kind, Fault,
// Retryability, StatusCode, Fields, and the values returned by Prop are
// Options.
type Option interface {
	applyTo(e *Error)
}

// Fault attributes an error to the party responsible for it.
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

var _ Option = FaultUnknown

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

func (f Fault) applyTo(e *Error) { e.fault = f }

// severity orders Faults for combining the children of a join: a join is
// attributed to the most severe Fault among its children.
func (f Fault) severity() int {
	switch f {
	case FaultInternal:
		return 3
	case FaultUpstream:
		return 2
	case FaultCaller:
		return 1
	default:
		return 0
	}
}

// Retryability reports whether retrying the operation that produced an error
// may succeed.
type Retryability uint8

const (
	UnknownRetryability Retryability = iota
	Retryable
	NonRetryable
)

var _ Option = UnknownRetryability

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

func (r Retryability) applyTo(e *Error) { e.retry = r }

// StatusCode is an explicit HTTP status code for an error, overriding the
// default of its Kind.
type StatusCode int

var _ Option = StatusCode(0)

func (c StatusCode) applyTo(e *Error) { e.status = c }

// Fields represents structured key-value pairs for logging. Fields never
// appear in Error(), %s, or %v; %+v prints them in logfmt form after the
// message, and LogValue exposes them to log/slog.
type Fields []any

var _ Option = Fields(nil)

// Add appends additional key-value pairs to the Fields slice.
// Can be called incrementally to build up context as it becomes available.
func (f *Fields) Add(fields ...any) {
	*f = append(*f, fields...)
}

// List returns the Fields as a flat slice of alternating keys and values.
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

func (f Fields) applyTo(e *Error) { e.fields = append(e.fields, f...) }

// prop is the Option returned by Prop.
type prop struct{ v any }

func (p prop) applyTo(e *Error) { e.props = append(e.props, p.v) }

// Prop returns an Option attaching v to an error as a custom property,
// retrievable with Lookup. Properties are distinguished by type: define a
// named type per property.
//
//	type TenantID string
//	err := errors.Wrap(err, "load tenant", errors.Prop(TenantID("acme")))
//	id, ok := errors.Lookup[TenantID](err)
func Prop(v any) Option { return prop{v} }

// Lookup returns the custom property of type T attached with Prop to the
// outermost layer of err's chain that has one. A property matches when it is
// assignable to T, so T may be an interface type. Searches an *Error's
// wrapped error before its cause and a join's children first to last.
func Lookup[T any](err error) (T, bool) {
	return find(err, func(err error) (T, bool) {
		var zero T
		e, ok := err.(*Error)
		if !ok {
			return zero, false
		}
		for i := len(e.props) - 1; i >= 0; i-- {
			if v, ok := e.props[i].(T); ok {
				return v, true
			}
		}
		return zero, false
	})
}

// apply applies items to e as documented on (*Error).Set.
func (e *Error) apply(items []any) {
	for _, item := range items {
		switch v := item.(type) {
		case Option:
			v.applyTo(e)
		case slog.Attr:
			e.fields = append(e.fields, v.Key, v.Value.Any())
		default:
			e.fields = append(e.fields, v)
		}
	}
}

type unwrapper interface {
	Unwrap() error
}

// multiUnwrapper is implemented by errors that join more than one error into
// a tree, such as the value returned by Join.
type multiUnwrapper interface {
	Unwrap() []error
}

// children returns the errors err directly wraps, in search order: an
// *Error's wrapped error then its cause, a join's children first to last, or
// the result of a single-error Unwrap.
func children(err error) []error {
	switch e := err.(type) {
	case *Error:
		var out []error
		if parent := normalizeError(e.parent); parent != nil {
			out = append(out, parent)
		}
		if cause := normalizeError(e.cause); cause != nil {
			out = append(out, cause)
		}
		return out
	case multiUnwrapper:
		return e.Unwrap()
	case unwrapper:
		if inner := normalizeError(e.Unwrap()); inner != nil {
			return []error{inner}
		}
	}
	return nil
}

func hasChildren(err error) bool {
	return len(children(err)) > 0
}

// find searches err's chain outermost first, an *Error's wrapped error
// before its cause and a join's children first to last, and returns the
// first value local reports for an error in the chain.
func find[T any](err error, local func(error) (T, bool)) (T, bool) {
	var zero T
	if isNilError(err) {
		return zero, false
	}
	if v, ok := local(err); ok {
		return v, true
	}
	for _, child := range children(err) {
		if v, ok := find(child, local); ok {
			return v, true
		}
	}
	return zero, false
}

// resolve searches err's chain like find, except at a join, where it
// resolves every child and combines their results with combine. found[i]
// reports whether child i's chain set the property.
func resolve[T any](err error, local func(error) (T, bool), combine func(vals []T, found []bool) (T, bool)) (T, bool) {
	var zero T
	if isNilError(err) {
		return zero, false
	}
	if v, ok := local(err); ok {
		return v, true
	}
	kids := children(err)
	if _, isJoin := err.(multiUnwrapper); isJoin {
		vals := make([]T, len(kids))
		found := make([]bool, len(kids))
		for i, child := range kids {
			vals[i], found[i] = resolve(child, local, combine)
		}
		return combine(vals, found)
	}
	for _, child := range kids {
		if v, ok := resolve(child, local, combine); ok {
			return v, true
		}
	}
	return zero, false
}

// GetFault returns the Fault of the outermost layer of err's chain that sets
// one, either explicitly or through the default of the Kind it sets (an
// explicit Fault on a layer overrides that layer's Kind). Searches an
// *Error's wrapped error before its cause. A join is attributed to the most
// severe Fault among its children: FaultInternal, then FaultUpstream, then
// FaultCaller. Returns FaultUnknown if no layer sets a Fault.
func GetFault(err error) Fault {
	f, _ := resolve(err, faultOf, func(vals []Fault, found []bool) (Fault, bool) {
		best := FaultUnknown
		for i, f := range vals {
			if found[i] && f.severity() > best.severity() {
				best = f
			}
		}
		return best, best != FaultUnknown
	})
	return f
}

func faultOf(err error) (Fault, bool) {
	switch e := err.(type) {
	case *Error:
		if e.fault != FaultUnknown {
			return e.fault, true
		}
		f := e.kind.Fault()
		return f, f != FaultUnknown
	case Kind:
		f := e.Fault()
		return f, f != FaultUnknown
	default:
		return FaultUnknown, false
	}
}

// GetRetryability returns the Retryability of the outermost layer of err's
// chain that sets one, either explicitly or through the default of the Kind
// it sets (an explicit Retryability on a layer overrides that layer's Kind).
// Searches an *Error's wrapped error before its cause. A join is NonRetryable
// if any child is NonRetryable, Retryable if every child is Retryable, and
// UnknownRetryability otherwise. Returns UnknownRetryability if no layer sets
// a Retryability.
func GetRetryability(err error) Retryability {
	r, _ := resolve(err, retryOf, func(vals []Retryability, found []bool) (Retryability, bool) {
		all := len(vals) > 0
		for i, r := range vals {
			if found[i] && r == NonRetryable {
				return NonRetryable, true
			}
			all = all && found[i] && r == Retryable
		}
		if all {
			return Retryable, true
		}
		return UnknownRetryability, false
	})
	return r
}

func retryOf(err error) (Retryability, bool) {
	switch e := err.(type) {
	case *Error:
		if e.retry != UnknownRetryability {
			return e.retry, true
		}
		r := e.kind.Retryability()
		return r, r != UnknownRetryability
	case Kind:
		r := e.Retryability()
		return r, r != UnknownRetryability
	default:
		return UnknownRetryability, false
	}
}

// IsRetryable reports whether GetRetryability(err) is Retryable.
func IsRetryable(err error) bool {
	return GetRetryability(err) == Retryable
}

// GetStatusCode returns the HTTP status code of the outermost layer of err's
// chain that sets one, either explicitly with StatusCode or through the
// default of the Kind it sets (an explicit StatusCode on a layer overrides
// that layer's Kind). Searches an *Error's wrapped error before its cause and
// a join's children first to last. Returns 0 if no layer sets a status code.
func GetStatusCode(err error) int {
	c, _ := find(err, func(err error) (int, bool) {
		switch e := err.(type) {
		case *Error:
			if e.status != 0 {
				return int(e.status), true
			}
			c := e.kind.StatusCode()
			return c, c != 0
		case Kind:
			c := e.StatusCode()
			return c, c != 0
		default:
			return 0, false
		}
	})
	return c
}

// GetFields returns every field in err's chain, outermost layer first: an
// *Error's own fields, then those of its wrapped error's chain, then those
// of its cause's chain; at a join, each child's fields in order. Fields are
// never deduplicated, since Fields.Lookup returns the first (outermost)
// match for a given key.
func GetFields(err error) Fields {
	var fields Fields
	collectFields(err, &fields)
	return fields
}

func collectFields(err error, out *Fields) {
	if isNilError(err) {
		return
	}
	if e, ok := err.(*Error); ok {
		*out = append(*out, e.fields...)
	}
	for _, child := range children(err) {
		collectFields(child, out)
	}
}

// ListFields returns GetFields(err) as a flat slice of alternating keys and
// values.
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

		key := fmt.Sprint(fields[i])
		io.WriteString(w, key)
		io.WriteString(w, "=")

		if i+1 < len(fields) {
			value := fields[i+1]
			switch v := value.(type) {
			case string:
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
