package utils

func PtrTo[T any](x T) *T {
	return &x
}

func ZeroIfNil[T any](x *T) T {
	var val T
	if x == nil {
		return val
	}
	return *x
}
