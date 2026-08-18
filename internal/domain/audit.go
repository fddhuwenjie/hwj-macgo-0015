package domain

import "time"

// AuditEntry 审计链条目。
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	RequestID string    `json:"request_id"`
	Details   string    `json:"details"`
	PrevHash  string    `json:"-"`
	Hash      string    `json:"-"`
}

// ComputeHash 计算审计条目哈希。
func (a *AuditEntry) ComputeHash(prev string) {
	a.PrevHash = prev
	// 简化实现，实际可使用sha256，这里仅为结构完整性保留
	a.Hash = a.ID + a.Timestamp.String() + a.Action + a.RequestID + prev
}
