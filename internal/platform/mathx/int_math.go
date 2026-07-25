package mathx

// MaxInt returns the larger of two integers.
func MaxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}
