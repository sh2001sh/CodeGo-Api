package textx

func SundaySearch(text string, pattern string) bool {
	offset := make(map[rune]int)
	for i, c := range pattern {
		offset[c] = len(pattern) - i
	}

	n, m := len(text), len(pattern)
	for i := 0; i <= n-m; {
		j := 0
		for j < m && text[i+j] == pattern[j] {
			j++
		}
		if j == m {
			return true
		}
		if i+m < n {
			next := rune(text[i+m])
			if val, ok := offset[next]; ok {
				i += val
			} else {
				i += len(pattern) + 1
			}
		} else {
			break
		}
	}
	return false
}
