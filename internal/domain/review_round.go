package domain

import "time"

// ReviewStatus 复核轮次状态。
type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "PENDING"
	ReviewApproved ReviewStatus = "APPROVED"
	ReviewRejected ReviewStatus = "REJECTED"
)

// ReviewRound 一次复核轮次。
type ReviewRound struct {
	ID              string       `json:"id"`
	RequestID       string       `json:"request_id"`
	RoundNumber     int          `json:"round_number"`
	ReviewerID      string       `json:"reviewer_id"`
	Status          ReviewStatus `json:"status"`
	Comment         string       `json:"comment"`
	ReviewedAt      *time.Time   `json:"reviewed_at,omitempty"`
	DecisionVersion string       `json:"decision_version,omitempty"` // 复核通过时冻结的条件版本ID
}

// Clone 返回拷贝。
func (r ReviewRound) Clone() ReviewRound {
	cp := r
	if r.ReviewedAt != nil {
		t := *r.ReviewedAt
		cp.ReviewedAt = &t
	}
	return cp
}
