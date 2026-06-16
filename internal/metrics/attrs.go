package metrics

func WordCountBucket(words int) string {
	switch {
	case words <= 0:
		return ""
	case words < 200:
		return "lt200"
	case words < 1000:
		return "200_999"
	default:
		return "gte1000"
	}
}
