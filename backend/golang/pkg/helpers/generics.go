package helpers

func Ptr[T any](value T) *T {
	return &value
}

func SafeDeref[T any](ptr *T) T {
	if ptr == nil {
		return *new(T)
	}
	return *ptr
}

func SafeLastN[T any](slice []T, lastN int) []T {
	if len(slice) > lastN {
		return slice[len(slice)-lastN:]
	}
	return slice
}
