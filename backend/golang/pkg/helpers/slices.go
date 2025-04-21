package helpers

func SafeSlice(slice []string, maxLength int) []string {
	if len(slice) > maxLength {
		return slice[:maxLength]
	}
	return slice
}
