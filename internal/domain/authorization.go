package domain

import "time"

// AuthorizationStatus 授权申请状态。
type AuthorizationStatus string

const (
	StatusDraft           AuthorizationStatus = "DRAFT"
	StatusConditionFrozen AuthorizationStatus = "CONDITION_FROZEN"
	StatusUnderReview     AuthorizationStatus = "UNDER_REVIEW"
	StatusEnabled         AuthorizationStatus = "ENABLED"
	StatusSuspended       AuthorizationStatus = "SUSPENDED"
	StatusResumed         AuthorizationStatus = "RESUMED"
	StatusExpired         AuthorizationStatus = "EXPIRED"
	StatusWithdrawn       AuthorizationStatus = "WITHDRAWN"
)

// AuthorizationRequest 授权申请聚合根。
type AuthorizationRequest struct {
	ID                string              `json:"id"`
	SubjectID         string              `json:"subject_id"`
	ResourceScope     ResourceScope       `json:"resource_scope"`
	PurposeConditions []PurposeCondition  `json:"purpose_conditions"`
	Status            AuthorizationStatus `json:"status"`
	CurrentVersionID  string              `json:"current_version_id"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	ExpiresAt         *time.Time          `json:"expires_at,omitempty"`
	Version           int64               `json:"version"`
	DelegationChainID string              `json:"delegation_chain_id,omitempty"`
}

// Clone 深拷贝。
func (a AuthorizationRequest) Clone() AuthorizationRequest {
	cp := a
	cp.ResourceScope = a.ResourceScope.Clone()
	conds := make([]PurposeCondition, len(a.PurposeConditions))
	for i, c := range a.PurposeConditions {
		conds[i] = c.Clone()
	}
	cp.PurposeConditions = conds
	if a.ExpiresAt != nil {
		t := *a.ExpiresAt
		cp.ExpiresAt = &t
	}
	return cp
}
