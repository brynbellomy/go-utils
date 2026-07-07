package errors

import "reflect"

const nilErrorString = "<nil>"

func isNilError(err error) bool {
	if err == nil {
		return true
	}

	v := reflect.ValueOf(err)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func normalizeError(err error) error {
	if isNilError(err) {
		return nil
	}
	return err
}
