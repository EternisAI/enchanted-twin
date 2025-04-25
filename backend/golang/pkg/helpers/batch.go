package helpers

func Batch[T any](items []T, batchSize int) [][]T {
	batches := make([][]T, 0)
	for i := 0; i < len(items); i += batchSize {
		end := min(i+batchSize, len(items))
		batches = append(batches, items[i:end])
	}
	return batches
}
