package domain

// 乐观并发控制版本辅助
func IncrementVersion(v int64) int64 {
	return v + 1
}
