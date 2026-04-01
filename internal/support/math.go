package support

func AbsInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
