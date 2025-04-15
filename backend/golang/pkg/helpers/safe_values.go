package helpers

func SafeValue[T any](value *T) T {
	if value == nil {
		return *new(T)
	}
	return *value
}

func Ptr[T any](value T) *T {
	return &value
}
