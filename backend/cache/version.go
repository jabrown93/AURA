package cache

func nextVersion(current, now int64) int64 {
	if current >= now {
		return current + 1
	}
	return now
}
