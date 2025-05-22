package helpers

func SafeFirstN[T any](slice []T, firstN int) []T {
	if len(slice) > firstN {
		return slice[:firstN]
	}
	return slice
}

func SafeLastN[T any](slice []T, lastN int) []T {
	if len(slice) > lastN {
		return slice[len(slice)-lastN:]
	}
	return slice
}
