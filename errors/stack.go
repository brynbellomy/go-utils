package errors

import (
	"runtime"

	pkgerrors "github.com/pkg/errors"
)

const maxStackDepth = 32

// callers returns the stack of the caller of the exported function or method
// that called callers. It must be called directly from that function.
func callers() []uintptr {
	var pcs [maxStackDepth]uintptr
	// Skip runtime.Callers, callers, and the exported function.
	n := runtime.Callers(3, pcs[:])
	return append([]uintptr(nil), pcs[:n]...)
}

// callersIfNone returns callers() if none of errs' chains carries a stack
// trace, and nil otherwise. It must be called directly from the exported
// function or method whose caller's stack is wanted.
func callersIfNone(errs ...error) []uintptr {
	for _, err := range errs {
		if hasStack(err) {
			return nil
		}
	}
	var pcs [maxStackDepth]uintptr
	// Skip runtime.Callers, callersIfNone, and the exported function.
	n := runtime.Callers(3, pcs[:])
	return append([]uintptr(nil), pcs[:n]...)
}

// hasOwnStack reports whether err itself, not counting its chain, carries a
// stack trace.
func hasOwnStack(err error) bool {
	switch e := err.(type) {
	case *Error:
		return e.stack != nil
	case stackTracer:
		return true
	default:
		return false
	}
}

// hasStack reports whether any error in err's chain carries a stack trace.
func hasStack(err error) bool {
	return stackOwner(err) != nil
}

// stackOwner returns the error carrying the stack trace deepest in err's
// chain, searching children in order (an *Error's wrapped error before its
// cause, a join's children first to last), or nil if no error in the chain
// carries one.
func stackOwner(err error) error {
	if isNilError(err) {
		return nil
	}
	for _, child := range children(err) {
		if owner := stackOwner(child); owner != nil {
			return owner
		}
	}
	if hasOwnStack(err) {
		return err
	}
	return nil
}

// traceOf returns the stack trace carried by owner itself, or nil.
func traceOf(owner error) pkgerrors.StackTrace {
	switch e := owner.(type) {
	case *Error:
		trace := make(pkgerrors.StackTrace, len(e.stack))
		for i, pc := range e.stack {
			trace[i] = pkgerrors.Frame(pc)
		}
		return trace
	case stackTracer:
		return e.StackTrace()
	default:
		return nil
	}
}
