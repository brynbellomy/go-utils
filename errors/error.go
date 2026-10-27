package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	pkgerrors "github.com/pkg/errors"
)

// Error is the package's single error node. Every constructor in the package
// returns an *Error (as error or as *Error), and every *Error implements the
// full set of interfaces that stdlib errors, github.com/pkg/errors, fmt, and
// log/slog look for: Error, Unwrap, Cause, Is, As, Format, StackTrace, and
// LogValue. Wrapping an *Error never hides any of them.
//
// An *Error is immutable once constructed. Its methods that return *Error
// (Set, Wrap, Wrapf, Stack, CausedBy) return a new node wrapping the receiver.
// Every constructor and method guarantees that the resulting chain carries a
// stack trace, capturing one at the call site when the chain has none.
//
// The methods returning *Error are nil-safe: called on a nil *Error they
// return nil. A nil *Error stored in an error interface is a non-nil error
// (Go interface semantics), so a chain started from a parent that may be nil
// ends with Err(), which returns an untyped nil error.
type Error struct {
	msg    string
	parent error
	cause  error
	stack  []uintptr
	kind   Kind
	fault  Fault
	retry  Retryability
	status StatusCode
	fields Fields
	props  []any
}

var (
	_ error                      = (*Error)(nil)
	_ fmt.Formatter              = (*Error)(nil)
	_ slog.LogValuer             = (*Error)(nil)
	_ stackTracer                = (*Error)(nil)
	_ interface{ Cause() error } = (*Error)(nil)
)

// stackTracer is the interface github.com/pkg/errors uses to expose stacks.
type stackTracer interface {
	StackTrace() pkgerrors.StackTrace
}

// New returns an error with the given message and a stack trace captured at
// the call site. items are applied as by (*Error).Set.
func New(msg string, items ...any) error {
	e := &Error{msg: msg, stack: callers()}
	e.apply(items)
	return e
}

// Sentinel returns an error with the given message and no stack trace, for
// declaring package-level sentinel values. Wrapping a sentinel captures a
// stack at the wrap site. items are applied as by (*Error).Set.
func Sentinel(msg string, items ...any) error {
	e := &Error{msg: msg}
	e.apply(items)
	return e
}

// Errorf returns an error with a message formatted per fmt.Errorf, including
// support for the %w verb, and a stack trace captured at the call site.
func Errorf(format string, args ...any) error {
	inner := fmt.Errorf(format, args...)
	e := &Error{stack: callers()}
	if hasChildren(inner) {
		e.parent = inner
	} else {
		e.msg = inner.Error()
	}
	return e
}

// Wrap returns an error annotating err with msg. A stack trace is captured at
// the call site unless err's chain already carries one. items are applied as
// by (*Error).Set. Wrap returns nil if err is nil or typed-nil.
func Wrap(err error, msg string, items ...any) error {
	if isNilError(err) {
		return nil
	}
	e := &Error{msg: msg, parent: err, stack: callersIfNone(err)}
	e.apply(items)
	return e
}

// Wrapf returns an error annotating err with a message formatted per
// fmt.Sprintf. A stack trace is captured at the call site unless err's chain
// already carries one. Wrapf returns nil if err is nil or typed-nil.
func Wrapf(err error, format string, args ...any) error {
	if isNilError(err) {
		return nil
	}
	return &Error{msg: fmt.Sprintf(format, args...), parent: err, stack: callersIfNone(err)}
}

// WithMessage returns an error annotating err with msg. It is Wrap without
// items, matching the github.com/pkg/errors API. Returns nil if err is nil or
// typed-nil.
func WithMessage(err error, msg string) error {
	if isNilError(err) {
		return nil
	}
	return &Error{msg: msg, parent: err, stack: callersIfNone(err)}
}

// WithMessagef returns an error annotating err with a message formatted per
// fmt.Sprintf. It is Wrapf, matching the github.com/pkg/errors API. Returns
// nil if err is nil or typed-nil.
func WithMessagef(err error, format string, args ...any) error {
	if isNilError(err) {
		return nil
	}
	return &Error{msg: fmt.Sprintf(format, args...), parent: err, stack: callersIfNone(err)}
}

// WithStack returns err wrapped in an *Error that carries a stack trace
// captured at the call site, unless err's chain already carries one, in which
// case the existing stack is kept. Returns nil if err is nil or typed-nil.
func WithStack(err error) error {
	if isNilError(err) {
		return nil
	}
	return &Error{parent: err, stack: callersIfNone(err)}
}

// WithMetadata returns err wrapped with properties and fields and no
// additional message. items are applied as by (*Error).Set. Returns nil if
// err is nil or typed-nil.
func WithMetadata(err error, items ...any) error {
	if isNilError(err) {
		return nil
	}
	e := &Error{parent: err, stack: callersIfNone(err)}
	e.apply(items)
	return e
}

// WithCause returns an error whose message is err's message followed by
// cause's, and whose chain contains both: Unwrap returns err, Cause returns
// cause, and Is, As, and every property and field lookup search err's chain
// first and cause's chain second. If either side is nil or typed-nil,
// WithCause returns the other side; if both are, it returns nil.
func WithCause(err error, cause error) error {
	err, cause = normalizeError(err), normalizeError(cause)
	if err == nil {
		return cause
	}
	if cause == nil {
		return err
	}
	return &Error{parent: err, cause: cause, stack: callersIfNone(err, cause)}
}

// With returns an *Error wrapping parent for fluent construction. If args are
// given, the first must be a string message, formatted per fmt.Sprintf with
// the remaining args. A stack trace is captured at the call site unless
// parent's chain already carries one. With returns nil if parent is nil or
// typed-nil.
func With(parent error, args ...any) *Error {
	if isNilError(parent) {
		return nil
	}
	return &Error{msg: formatArgs(args), parent: parent, stack: callersIfNone(parent)}
}

// WithNew returns an *Error for fluent construction from either a string
// message (formatted per fmt.Sprintf with args when any are given) or an
// error to wrap (args are ignored). A stack trace is captured at the call
// site unless the wrapped error's chain already carries one. Returns nil if
// parentOrMsg is a nil or typed-nil error. Panics if parentOrMsg is neither a
// string nor an error.
func WithNew(parentOrMsg any, args ...any) *Error {
	switch x := parentOrMsg.(type) {
	case string:
		if len(args) > 0 {
			x = fmt.Sprintf(x, args...)
		}
		return &Error{msg: x, stack: callers()}
	case error:
		if isNilError(x) {
			return nil
		}
		return &Error{parent: x, stack: callersIfNone(x)}
	default:
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", parentOrMsg))
	}
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	msg, ok := args[0].(string)
	if !ok {
		panic(fmt.Sprintf("invariant violation: got %T, expected string message", args[0]))
	}
	if len(args) > 1 {
		return fmt.Sprintf(msg, args[1:]...)
	}
	return msg
}

// Set returns a new *Error wrapping e with the given items applied. Each item
// is one of:
//   - an Option (Kind, Fault, Retryability, StatusCode, Fields, or a value
//     returned by Prop), which sets that property or appends those fields;
//   - a slog.Attr, which appends its key and value as a field;
//   - any other value, which is appended to the fields as-is, so
//     consecutive "key", value items form a field.
//
// Returns nil if e is nil.
func (e *Error) Set(items ...any) *Error {
	if e == nil {
		return nil
	}
	n := &Error{parent: e, stack: callersIfNone(e)}
	n.apply(items)
	return n
}

// Wrap returns a new *Error annotating e with msg. Returns nil if e is nil.
func (e *Error) Wrap(msg string) *Error {
	if e == nil {
		return nil
	}
	return &Error{msg: msg, parent: e, stack: callersIfNone(e)}
}

// Wrapf returns a new *Error annotating e with a message formatted per
// fmt.Sprintf. Returns nil if e is nil.
func (e *Error) Wrapf(format string, args ...any) *Error {
	if e == nil {
		return nil
	}
	return &Error{msg: fmt.Sprintf(format, args...), parent: e, stack: callersIfNone(e)}
}

// Stack returns e, wrapped in a new *Error carrying a stack trace captured at
// the call site if e's chain does not already carry one. Returns nil if e is
// nil.
func (e *Error) Stack() *Error {
	if e == nil {
		return nil
	}
	if hasStack(e) {
		return e
	}
	return &Error{parent: e, stack: callers()}
}

// CausedBy returns a new *Error recording cause as the cause of e, as by
// WithCause. Returns e if cause is nil or typed-nil, and nil if e is nil.
func (e *Error) CausedBy(cause error) *Error {
	if e == nil {
		return nil
	}
	if isNilError(cause) {
		return e
	}
	return &Error{parent: e, cause: cause, stack: callersIfNone(e, cause)}
}

// Err returns e as an error, or an untyped nil error if e is nil.
func (e *Error) Err() error {
	if e == nil {
		return nil
	}
	return e
}

// Error returns the message chain: each layer's message, outermost first,
// joined by ": ". Fields and properties never appear in Error().
func (e *Error) Error() string {
	if e == nil {
		return nilErrorString
	}
	msg := e.msg
	if parent := normalizeError(e.parent); parent != nil {
		msg = joinMessages(msg, parent.Error())
	}
	if cause := normalizeError(e.cause); cause != nil {
		msg = joinMessages(msg, cause.Error())
	}
	return msg
}

func joinMessages(outer, inner string) string {
	switch {
	case outer == "":
		return inner
	case inner == "":
		return outer
	default:
		return outer + ": " + inner
	}
}

// Unwrap returns the error e wraps, or nil.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return normalizeError(e.parent)
}

// Cause implements the causer interface of github.com/pkg/errors: it returns
// the cause recorded by WithCause or CausedBy if there is one, and the
// wrapped error otherwise, so that Cause traverses through every *Error.
func (e *Error) Cause() error {
	if e == nil {
		return nil
	}
	if cause := normalizeError(e.cause); cause != nil {
		return cause
	}
	return normalizeError(e.parent)
}

// Is reports whether target is the Kind set on e, or matches any error in
// the chain of the cause recorded by WithCause or CausedBy. errors.Is
// searches e's wrapped chain itself.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	if k, ok := target.(Kind); ok && e.kind != KindUnknown && e.kind == k {
		return true
	}
	if cause := normalizeError(e.cause); cause != nil {
		return stderrors.Is(cause, target)
	}
	return false
}

// As reports whether any error in the chain of the cause recorded by
// WithCause or CausedBy matches target, setting target if so. errors.As
// searches e's wrapped chain itself.
func (e *Error) As(target any) bool {
	if e == nil {
		return false
	}
	if cause := normalizeError(e.cause); cause != nil {
		return stderrors.As(cause, target)
	}
	return false
}

// StackTrace returns the stack trace recorded deepest in e's chain, which
// is the one captured closest to where the error originated. It implements
// the stackTracer interface of github.com/pkg/errors.
func (e *Error) StackTrace() pkgerrors.StackTrace {
	if e == nil {
		return nil
	}
	return traceOf(stackOwner(e))
}

// Format implements fmt.Formatter. %s and %v print Error(); %q prints
// Error() quoted; %+v prints Error(), then every field in the chain in
// logfmt form, then the stack trace returned by StackTrace.
func (e *Error) Format(s fmt.State, verb rune) {
	if e == nil {
		io.WriteString(s, nilErrorString)
		return
	}
	switch {
	case verb == 'v' && s.Flag('+'):
		io.WriteString(s, e.Error())
		if fields := GetFields(e); len(fields) > 0 {
			io.WriteString(s, " ")
			formatLogfmtFields(s, fields)
		}
		if trace := e.StackTrace(); len(trace) > 0 {
			fmt.Fprintf(s, "%+v", trace)
		}
	case verb == 'q':
		io.WriteString(s, strconv.Quote(e.Error()))
	default:
		io.WriteString(s, e.Error())
	}
}

// LogValue implements slog.LogValuer. It returns a group holding the message
// as "msg", the kind, fault, and retryability as "kind", "fault", and
// "retryability" when known, and every field in the chain. Field keys that
// are not strings are formatted with fmt.Sprint; a dangling key is paired
// with the value "!MISSING".
func (e *Error) LogValue() slog.Value {
	if e == nil {
		return slog.StringValue(nilErrorString)
	}
	attrs := []slog.Attr{slog.String("msg", e.Error())}
	if k := GetKind(e); k != KindUnknown {
		attrs = append(attrs, slog.String("kind", k.String()))
	}
	if f := GetFault(e); f != FaultUnknown {
		attrs = append(attrs, slog.String("fault", f.String()))
	}
	if r := GetRetryability(e); r != UnknownRetryability {
		attrs = append(attrs, slog.String("retryability", r.String()))
	}
	fields := GetFields(e)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			key = fmt.Sprint(fields[i])
		}
		var val any = "!MISSING"
		if i+1 < len(fields) {
			val = fields[i+1]
		}
		attrs = append(attrs, slog.Any(key, val))
	}
	return slog.GroupValue(attrs...)
}
