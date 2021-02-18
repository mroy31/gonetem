package utils

func Index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}
