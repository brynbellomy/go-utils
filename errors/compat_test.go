package errors_test

import (
	stderrors "errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	pkgerrors "github.com/pkg/errors"

	"github.com/brynbellomy/go-utils/errors"
)

// The compatibility suite composes random chains of every constructor this
// package offers, interleaved with stdlib and github.com/pkg/errors wrappers,
// and checks a fixed set of invariants against each chain. Any invariant
// listed in knownViolations is reported with t.Log instead of failing.

type rootError struct{ msg string }

func (e *rootError) Error() string { return e.msg }

// layer is one wrapping step in a composed chain.
type layer struct {
	name string
	// wrap wraps err. i is the layer's position in the chain, used to make
	// field keys unique.
	wrap func(err error, i int) error
	// ours is true when the outermost value is produced by this package, so
	// formatting, stack, and causer guarantees apply.
	ours bool
	// causer is true when github.com/pkg/errors.Cause traverses this layer
	// toward the wrapped error.
	causer bool
	// fault is the Fault this layer sets, or FaultUnknown. compose assigns it
	// for layers that set fields, which set faultFor(i) alongside.
	fault errors.Fault
	// field is true when this layer sets the field fieldKey(i).
	field bool
	// nilSafe is true when wrapping a nil or typed-nil error must return an
	// untyped nil. github.com/pkg/errors does not detect typed-nil errors, so
	// its layers are not nilSafe.
	nilSafe bool
}

func fieldKey(i int) string { return "suitekey_" + strconv.Itoa(i) }

var other = stderrors.New("other")

func faultFor(i int) errors.Fault {
	if i%2 == 0 {
		return errors.FaultInternal
	}
	return errors.FaultCaller
}

func layers() []layer {
	return []layer{
		{name: "Wrap", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, _ int) error { return errors.Wrap(err, "wrap") }},
		{name: "Wrapf", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, i int) error { return errors.Wrapf(err, "wrapf %d", i) }},
		{name: "WithMessage", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, _ int) error { return errors.WithMessage(err, "msg") }},
		{name: "WithStack", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, _ int) error { return errors.WithStack(err) }},
		{name: "WithMetadata", ours: true, causer: true, field: true, nilSafe: true,
			wrap: func(err error, i int) error {
				return errors.WithMetadata(err, faultFor(i), fieldKey(i), i)
			}},
		{name: "With.Set.Err", ours: true, causer: true, field: true, nilSafe: true,
			wrap: func(err error, i int) error {
				return errors.With(err, "with").Set(faultFor(i), fieldKey(i), i).Err()
			}},
		{name: "With.Set", ours: true, causer: true, field: true,
			wrap: func(err error, i int) error {
				return errors.With(err, "with").Set(faultFor(i), fieldKey(i), i)
			}},
		{name: "With.Stack", ours: true, causer: true,
			wrap: func(err error, _ int) error { return errors.With(err).Stack() }},
		{name: "Wrap+items", ours: true, causer: true, field: true, nilSafe: true,
			wrap: func(err error, i int) error {
				return errors.Wrap(err, "items", errors.ErrNotFound, faultFor(i), fieldKey(i), i)
			}},
		{name: "Errorf(%w)", ours: true,
			wrap: func(err error, _ int) error { return errors.Errorf("errorf: %w", err) }},
		{name: "With.CausedBy(err)", ours: true, causer: true,
			wrap: func(err error, _ int) error { return errors.With(other, "with").CausedBy(err) }},
		{name: "WithCause(err,other)", ours: true, nilSafe: false,
			wrap: func(err error, _ int) error { return errors.WithCause(err, other) }},
		{name: "WithCause(other,err)", ours: true, causer: true,
			wrap: func(err error, _ int) error { return errors.WithCause(other, err) }},
		{name: "Annotate", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, _ int) error { errors.Annotate(&err, "annotated"); return err }},
		{name: "AddStack", ours: true, causer: true, nilSafe: true,
			wrap: func(err error, _ int) error { errors.AddStack(&err); return err }},
		{name: "fmt.Errorf", nilSafe: false,
			wrap: func(err error, _ int) error { return fmt.Errorf("fmt: %w", err) }},
		{name: "pkg.Wrap", causer: true,
			wrap: func(err error, _ int) error { return pkgerrors.Wrap(err, "pkg") }},
		{name: "pkg.WithStack", causer: true,
			wrap: func(err error, _ int) error { return pkgerrors.WithStack(err) }},
		{name: "Join(err,other)",
			wrap: func(err error, _ int) error { return errors.Join(err, other) }},
		{name: "Join(other,err)",
			wrap: func(err error, _ int) error { return errors.Join(other, err) }},
	}
}

// chain is a composed sequence of layers applied innermost first.
type chain []layer

func (c chain) String() string {
	names := make([]string, len(c))
	for i, l := range c {
		names[i] = l.name
	}
	return strings.Join(names, " > ")
}

func (c chain) build(root error) error {
	err := root
	for i, l := range c {
		err = l.wrap(err, i)
	}
	return err
}

func (c chain) allCausers() bool {
	for _, l := range c {
		if !l.causer {
			return false
		}
	}
	return true
}

func (c chain) expectedFault() errors.Fault {
	for i := len(c) - 1; i >= 0; i-- {
		if c[i].fault != errors.FaultUnknown {
			return c[i].fault
		}
	}
	return errors.FaultUnknown
}

// invariant is a named property that every composed chain must satisfy.
type invariant struct {
	name  string
	check func(c chain, root *rootError, err error) error
}

var invariants = []invariant{
	{"Is reaches the root", func(_ chain, root *rootError, err error) error {
		if !errors.Is(err, root) {
			return stderrors.New("errors.Is(err, root) is false")
		}
		return nil
	}},
	{"As reaches the root", func(_ chain, root *rootError, err error) error {
		var target *rootError
		if !errors.As(err, &target) || target != root {
			return stderrors.New("errors.As did not find the root")
		}
		return nil
	}},
	{"pkg/errors.Cause reaches the root", func(c chain, root *rootError, err error) error {
		if !c.allCausers() {
			return nil
		}
		if got := pkgerrors.Cause(err); got != error(root) {
			return fmt.Errorf("pkgerrors.Cause returned %T %q", got, got)
		}
		return nil
	}},
	{"Cause reaches the root", func(c chain, root *rootError, err error) error {
		if !c.allCausers() {
			return nil
		}
		if got := errors.Cause(err); got != error(root) {
			return fmt.Errorf("errors.Cause returned %T %q", got, got)
		}
		return nil
	}},
	{"OneOf matches the root", func(_ chain, root *rootError, err error) error {
		if !errors.OneOf(err, root) {
			return stderrors.New("OneOf(err, root) is false")
		}
		return nil
	}},
	{"%+v prints exactly one stack", func(c chain, _ *rootError, err error) error {
		if !c[len(c)-1].ours {
			return nil
		}
		if n := strings.Count(fmt.Sprintf("%+v", err), "runtime.goexit"); n != 1 {
			return fmt.Errorf("%%+v printed %d stacks", n)
		}
		return nil
	}},
	{"stack originates at a call site in this file", func(c chain, _ *rootError, err error) error {
		st, ok := err.(interface{ StackTrace() pkgerrors.StackTrace })
		if !ok || len(st.StackTrace()) == 0 {
			return nil
		}
		if top := fmt.Sprintf("%+v", st.StackTrace()[0]); !strings.Contains(top, "compat_test.go") {
			return fmt.Errorf("top frame is %s", top)
		}
		return nil
	}},
	{"Kind is found", func(c chain, _ *rootError, err error) error {
		for _, l := range c {
			if l.name == "Wrap+items" {
				if !errors.Is(err, errors.ErrNotFound) || errors.GetKind(err) != errors.ErrNotFound {
					return fmt.Errorf("Is = %v, GetKind = %v", errors.Is(err, errors.ErrNotFound), errors.GetKind(err))
				}
			}
		}
		return nil
	}},
	{"StackTrace is exposed", func(c chain, _ *rootError, err error) error {
		if !c[len(c)-1].ours {
			return nil
		}
		st, ok := err.(interface{ StackTrace() pkgerrors.StackTrace })
		if !ok {
			return fmt.Errorf("%T does not implement StackTrace", err)
		}
		if len(st.StackTrace()) == 0 {
			return stderrors.New("StackTrace is empty")
		}
		return nil
	}},
	{"%s, %v and %q agree with Error()", func(c chain, _ *rootError, err error) error {
		if !c[len(c)-1].ours {
			return nil
		}
		msg := err.Error()
		if s := fmt.Sprintf("%s", err); s != msg {
			return fmt.Errorf("%%s = %q, Error() = %q", s, msg)
		}
		if v := fmt.Sprintf("%v", err); v != msg {
			return fmt.Errorf("%%v = %q, Error() = %q", v, msg)
		}
		if q := fmt.Sprintf("%q", err); q != strconv.Quote(msg) {
			return fmt.Errorf("%%q = %s, want %s", q, strconv.Quote(msg))
		}
		return nil
	}},
	{"Error() contains no fields", func(_ chain, _ *rootError, err error) error {
		if strings.Contains(err.Error(), "suitekey") {
			return fmt.Errorf("Error() = %q", err.Error())
		}
		return nil
	}},
	{"every field is found", func(c chain, _ *rootError, err error) error {
		for i, l := range c {
			if !l.field {
				continue
			}
			v, ok := errors.LookupField(err, fieldKey(i))
			if !ok || v != i {
				return fmt.Errorf("field %s: got (%v, %v)", fieldKey(i), v, ok)
			}
		}
		return nil
	}},
	{"outermost fault wins", func(c chain, _ *rootError, err error) error {
		if got, want := errors.GetFault(err), c.expectedFault(); got != want {
			return fmt.Errorf("GetFault = %v, want %v", got, want)
		}
		return nil
	}},
}

// knownViolations lists invariants the package does not yet satisfy, keyed by
// invariant name, with the reason. Failures of these invariants are logged,
// not failed.
var knownViolations = map[string]string{}

// compose returns a chain of the given layers, with each fault-setting layer's
// expected fault resolved for its position.
func compose(ls ...layer) chain {
	c := make(chain, len(ls))
	for i, l := range ls {
		if l.field {
			l.fault = faultFor(i)
		}
		c[i] = l
	}
	return c
}

func chains() []chain {
	ls := layers()
	var out []chain
	for _, a := range ls {
		out = append(out, compose(a))
		for _, b := range ls {
			out = append(out, compose(a, b))
		}
	}
	rng := rand.New(rand.NewSource(1))
	for range 500 {
		depth := 3 + rng.Intn(4)
		picked := make([]layer, depth)
		for i := range picked {
			picked[i] = ls[rng.Intn(len(ls))]
		}
		out = append(out, compose(picked...))
	}
	return out
}

func TestCompat_Invariants(t *testing.T) {
	all := chains()
	for _, inv := range invariants {
		t.Run(inv.name, func(t *testing.T) {
			var failures []string
			for _, c := range all {
				root := &rootError{msg: "root"}
				if err := inv.check(c, root, c.build(root)); err != nil {
					failures = append(failures, fmt.Sprintf("%s: %v", c, err))
				}
			}
			if len(failures) == 0 {
				return
			}
			report := fmt.Sprintf("%d of %d chains fail; first: %s", len(failures), len(all), failures[0])
			if reason, known := knownViolations[inv.name]; known {
				t.Logf("known violation (%s): %s", reason, report)
				return
			}
			t.Error(report)
		})
	}
}

func TestCompat_KnownViolationsAreCurrent(t *testing.T) {
	names := make(map[string]bool, len(invariants))
	for _, inv := range invariants {
		names[inv.name] = true
	}
	for name := range knownViolations {
		if !names[name] {
			t.Errorf("knownViolations names unknown invariant %q", name)
		}
	}
}

func TestCompat_NilInputs(t *testing.T) {
	var typedNil *rootError
	for _, l := range layers() {
		if !l.nilSafe {
			continue
		}
		t.Run(l.name, func(t *testing.T) {
			var failures []string
			if err := l.wrap(nil, 0); err != nil {
				failures = append(failures, fmt.Sprintf("nil input returned %T", err))
			}
			if err := l.wrap(typedNil, 0); err != nil {
				failures = append(failures, fmt.Sprintf("typed-nil input returned %T", err))
			}
			if len(failures) == 0 {
				return
			}
			t.Error(strings.Join(failures, "; "))
		})
	}
}
