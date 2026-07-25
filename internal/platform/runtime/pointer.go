package runtime

// GetPointer returns a pointer to the provided value.
func GetPointer[T any](v T) *T {
	return &v
}
