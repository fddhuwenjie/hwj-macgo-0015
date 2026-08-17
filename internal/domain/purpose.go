package domain

import "time"

// PurposeCondition 描述授权的用途条件。
type PurposeCondition struct {
	Description string    `json:"description"`
	ValidFrom   time.Time `json:"valid_from"`
	ValidUntil  time.Time `json:"valid_until"`
}

// Clone 返回拷贝。
func (p PurposeCondition) Clone() PurposeCondition {
	return p
}

// IsActiveAt 判断在指定时间是否有效。
func (p PurposeCondition) IsActiveAt(t time.Time) bool {
	return !t.Before(p.ValidFrom) && t.Before(p.ValidUntil)
}
