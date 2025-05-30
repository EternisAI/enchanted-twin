package helpers

func SafeLastN[T any](slice []T, lastN int) []T {
	if len(slice) > lastN {
		return slice[len(slice)-lastN:]
	}
	return slice
}
