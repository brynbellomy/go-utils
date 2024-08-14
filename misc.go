package utils

func PtrTo[T any](x T) *T {
	return &x
}
