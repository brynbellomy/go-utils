package errors

import (
	"github.com/pkg/errors"
)

type Fault uint8

const (
	FaultUnknown Fault = 1
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

type withProperties struct {
	fault      Fault
	statusCode StatusCode
	retryable  Retryability
	parent     error
}

func WithProperties(err error, props ...any) error {
	if err == nil {
		return nil
	}

	wp := &withProperties{
		parent: err,
	}

	for _, prop := range props {
		switch v := prop.(type) {
		case Fault:
			wp.fault = v
		case StatusCode:
			wp.statusCode = v
		case Retryability:
			wp.retryable = v
		}
	}

	return wp
}

func (ef *withProperties) Error() string {
	if ef.parent != nil {
		return ef.parent.Error()
	}
	return "error with properties"
}

func IsRetryable(err error) bool {
	wp := &withProperties{}
	if !errors.As(err, &wp) {
		return false
	}
	return wp.retryable == Retryable
}

func GetStatusCode(err error) int {
	wp := &withProperties{}
	if !errors.As(err, &wp) {
		return 0
	}
	return int(wp.statusCode)
}

func GetFault(err error) Fault {
	wp := &withProperties{}
	if !errors.As(err, &wp) {
		return FaultUnknown
	}
	return wp.fault
}
