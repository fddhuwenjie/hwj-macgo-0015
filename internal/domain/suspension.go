package domain

import "time"

// TemporarySuspension 临时暂停记录。
type TemporarySuspension struct {
	ID              string     `json:"id"`
	RequestID       string     `json:"request_id"`
	Reason          string     `json:"reason"`
	SuspendedAt     time.Time  `json:"suspended_at"`
	ResumeAt        *time.Time `json:"resume_at,omitempty"`
	ResumedAt       *time.Time `json:"resumed_at,omitempty"`
}

// Clone 拷贝。
func (t TemporarySuspension) Clone() TemporarySuspension {
	cp := t
	if t.ResumeAt != nil {
		v := *t.ResumeAt
		cp.ResumeAt = &v
	}
	if t.ResumedAt != nil {
		v := *t.ResumedAt
		cp.ResumedAt = &v
	}
	return cp
}
