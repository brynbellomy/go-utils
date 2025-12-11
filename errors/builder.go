package errors

import "fmt"

type Builder struct {
	error
}

func With(parent error, args ...any) *Builder {
	if parent == nil {
		return nil
	} else if len(args) == 0 {
		return &Builder{error: parent}
	}

	if msg, isStr := args[0].(string); !isStr {
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", msg))
	} else {
		if len(args) > 1 {
			msg = fmt.Sprintf(msg, args[1:]...)
		}
		return &Builder{error: Wrap(parent, msg)}
	}
}

func WithNew(parentOrMsg any, args ...any) *Builder {
	var err error
	switch x := parentOrMsg.(type) {
	case string:
		err = New(x)
	case error:
		err = x
	default:
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", parentOrMsg))
	}
	return &Builder{error: err}
}

func (b *Builder) Err() error {
	if b == nil {
		return nil
	}
	return b.error
}

func (b *Builder) Wrap(msg string) *Builder {
	if b == nil {
		return nil
	}
	b.error = Wrap(b.error, msg)
	return b
}

func (b *Builder) Wrapf(msg string, args ...any) *Builder {
	if b == nil {
		return nil
	}
	b.error = Wrapf(b.error, msg, args...)
	return b
}

func (b *Builder) Cause(cause error) *Builder {
	if b == nil {
		return nil
	}
	b.error = WithCause(b.error, cause)
	return b
}

func (b *Builder) Props(props ...any) *Builder {
	if b == nil {
		return nil
	}
	b.error = WithProperties(b.error, props...)
	return b
}

func (b *Builder) Fields(fields ...any) *Builder {
	if b == nil {
		return nil
	}
	b.error = WithFields(b.error, fields...)
	return b
}

func (b *Builder) Stack() *Builder {
	if b == nil {
		return nil
	}
	b.error = WithStack(b.error)
	return b
}

func (b *Builder) Set(things ...any) *Builder {
	for _, thing := range things {
		switch v := thing.(type) {
		case Fault, StatusCode, Retryability:
			b = b.Props(v)
		case Fields:
			b = b.Fields(v...)
		default:
			panic(fmt.Sprintf("invariant violation: got %T, expected error, string, Fault, StatusCode, Retryability, or Fields", thing))
		}
	}
	return b
}
