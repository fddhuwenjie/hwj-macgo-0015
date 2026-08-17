package domain

import "time"

// 一些时间辅助函数
func Now() time.Time {
	return time.Now().UTC()
}
