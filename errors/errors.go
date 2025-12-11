package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// StatusCoder represents an error with an associated HTTP status code
type StatusCoder struct {
	Code    int
	Message string
}

func (e *StatusCoder) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func NewStatusCoder(code int, message string) *StatusCoder {
	return &StatusCoder{
		Code:    code,
		Message: message,
	}
}

func AsStatusCoder(err error) (*StatusCoder, bool) {
	var httpErr *StatusCoder
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

func IsStatusCoder(err error, code ...int) bool {
	var httpErr *StatusCoder
	if errors.As(err, &httpErr) {
		if len(code) == 0 {
			return true
		}
		for _, c := range code {
			if httpErr.Code == c {
				return true
			}
		}
	}
	return false
}

// HTTP-style errors (useful elsewhere too)
var (
	ErrBadRequest          = NewStatusCoder(http.StatusBadRequest, "bad Request")
	ErrUnauthorized        = NewStatusCoder(http.StatusUnauthorized, "unauthorized")
	ErrForbidden           = NewStatusCoder(http.StatusForbidden, "forbidden")
	ErrNotFound            = NewStatusCoder(http.StatusNotFound, "not found")
	ErrTooManyRequests     = NewStatusCoder(http.StatusTooManyRequests, "too many requests")
	ErrInternalServerError = NewStatusCoder(http.StatusInternalServerError, "internal server error")
	ErrUnimplemented       = NewStatusCoder(http.StatusNotImplemented, "not implemented")
	ErrServiceUnavailable  = NewStatusCoder(http.StatusServiceUnavailable, "service unavailable")
)

// Other non-HTTP errors
var (
	ErrConnection  = errors.New("connection failed")
	ErrClosed      = errors.New("closed")
	ErrUnsupported = errors.New("unsupported")
)

var (
	New          = errors.New
	Errorf       = errors.Errorf
	Wrap         = errors.Wrap
	Wrapf        = errors.Wrapf
	WithStack    = errors.WithStack
	WithMessage  = errors.WithMessage
	WithMessagef = errors.WithMessagef
	Cause        = errors.Cause
	Join         = stderrors.Join
)

func Annotate(err *error, msg string, args ...any) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func AddStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}

func OneOf(received error, errs ...error) bool {
	for _, err := range errs {
		if Cause(received) == err {
			return true
		}
	}
	return false
}

func WithCause(err error, cause error) error {
	return &withCause{err, cause}
}

type withCause struct {
	error
	cause error
}

func (w *withCause) Error() string { return w.error.Error() + ": " + w.cause.Error() }

func (w *withCause) Cause() error { return w.cause }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withCause) Unwrap() error { return w.cause }

func (w *withCause) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", w.Cause())
			io.WriteString(s, w.error.Error())
			return
		}
		fallthrough
	case 's', 'q':
		io.WriteString(s, w.Error())
	}
}

type Fields []any

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

type withFields struct {
	fields Fields
	parent error
}

func NewWithFields(msg string, fields ...any) error {
	return WithFields(New(msg), fields...)
}

func WrapWithFields(err error, msg string, fields ...any) error {
	if err == nil {
		return nil
	}
	return WithFields(Wrap(err, msg), fields...)
}

func WithFields(err error, fields ...any) error {
	if err == nil {
		return nil
	}

	flattened := make(Fields, 0, len(fields))
	for _, x := range fields {
		if fs, isFields := x.(Fields); isFields {
			flattened = append(flattened, fs...)
		} else {
			flattened = append(flattened, x)
		}
	}

	return &withFields{
		parent: err,
		fields: flattened,
	}
}

func GetFields(err error) Fields {
	var fields []any
	for {
		errf := &withFields{}
		if !errors.As(err, &errf) {
			break
		}
		fields = append(fields, errf.fields...)
		err = errf.parent
	}
	return fields
}

func ListFields(err error) []any {
	return GetFields(err)
}

func (ef *withFields) Error() string {
	if ef.parent != nil {
		return ef.parent.Error()
	}
	return "error with fields"
}

func (ef *withFields) Unwrap() error {
	return ef.parent
}

func (ef *withFields) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// Verbose format: show parent error with stack if available
			if ef.parent != nil {
				fmt.Fprintf(s, "%+v", ef.parent)
			} else {
				io.WriteString(s, "error with fields")
			}
		} else {
			// Standard %v format
			io.WriteString(s, ef.Error())
		}
		// Add all fields in logfmt format
		allFields := GetFields(ef)
		if len(allFields) > 0 {
			io.WriteString(s, " ")
			formatLogfmt(s, allFields)
		}
	case 's':
		io.WriteString(s, ef.Error())
		// Add all fields in logfmt format
		allFields := GetFields(ef)
		if len(allFields) > 0 {
			io.WriteString(s, " ")
			formatLogfmt(s, allFields)
		}
	case 'q':
		// For quoted format, just quote the error message without fields
		fmt.Fprintf(s, "%q", ef.Error())
	}
}

func formatLogfmt(w io.Writer, fields []any) {
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
