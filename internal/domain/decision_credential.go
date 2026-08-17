package domain

import "time"

// DecisionType 决策类型。
type DecisionType string

const (
	DecisionApprove DecisionType = "APPROVE"
	DecisionReject  DecisionType = "REJECT"
	DecisionEnable  DecisionType = "ENABLE"
	DecisionSuspend DecisionType = "SUSPEND"
	DecisionResume  DecisionType = "RESUME"
	DecisionExpire  DecisionType = "EXPIRE"
	DecisionWithdraw DecisionType = "WITHDRAW"
)

// DecisionCredential 决策凭据，记录不可变决策。
type DecisionCredential struct {
	ID               string       `json:"id"`
	RequestID        string       `json:"request_id"`
	Type             DecisionType `json:"type"`
	ConditionVersion string       `json:"condition_version"` // 冻结的条件版本ID
	ScopeSnapshot    ResourceScope `json:"scope_snapshot"`
	DecidedAt        time.Time    `json:"decided_at"`
	DecidedBy        string       `json:"decided_by"`
	Signature        string       `json:"signature"` // 简单签名
	IdempotencyKey   string       `json:"idempotency_key"`
}

// Clone 深拷贝。
func (d DecisionCredential) Clone() DecisionCredential {
	cp := d
	cp.ScopeSnapshot = d.ScopeSnapshot.Clone()
	return cp
}
