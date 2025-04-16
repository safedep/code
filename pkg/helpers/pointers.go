package helpers

func PtrTo[T any](v T) *T {
	return &v
}
