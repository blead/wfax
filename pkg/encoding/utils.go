package encoding

func Flatten[S ~[]E, E any](slices []S) []E {
	total := 0
	for _, slice := range slices {
		total += len(slice)
	}
	flat := make([]E, total)
	i := 0
	for _, slice := range slices {
		i += copy(flat[i:], slice)
	}
	return flat
}
