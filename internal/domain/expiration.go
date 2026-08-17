package domain

import "time"

// ExpirationStatus 到期事件状态。
type ExpirationStatus string

const (
	ExpirationPending   ExpirationStatus = "PENDING"
	ExpirationProcessed ExpirationStatus = "PROCESSED"
	ExpirationMissed    ExpirationStatus = "MISSED"
)

// ExpirationEvent 到期事件。
type ExpirationEvent struct {
	ID           string           `json:"id"`
	RequestID    string           `json:"request_id"`
	ExpiresAt    time.Time        `json:"expires_at"`
	Status       ExpirationStatus `json:"status"`
	ProcessedAt  *time.Time       `json:"processed_at,omitempty"`
}

// Clone 拷贝。
func (e ExpirationEvent) Clone() ExpirationEvent {
	cp := e
	if e.ProcessedAt != nil {
		t := *e.ProcessedAt
		cp.ProcessedAt = &t
	}
	return cp
}
