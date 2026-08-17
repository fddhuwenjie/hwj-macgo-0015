package domain

import (
	"fmt"
	"strings"
	"time"
)

// ValidateSubject 验证主体数据完整性。
func ValidateSubject(s Subject) error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: subject id required", ErrInvalidArgument)
	}
	if len(s.ID) > 128 {
		return fmt.Errorf("%w: subject id too long", ErrInvalidArgument)
	}
	if s.Version < 0 {
		return fmt.Errorf("%w: subject version must be non-negative", ErrInvalidArgument)
	}
	if !s.CreatedAt.IsZero() && !s.UpdatedAt.IsZero() && s.UpdatedAt.Before(s.CreatedAt) {
		return fmt.Errorf("%w: updated_at before created_at", ErrInvalidArgument)
	}
	return nil
}

// ValidateAuthorizationRequest 校验授权申请的不变量。
func ValidateAuthorizationRequest(r AuthorizationRequest) error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: request id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(r.SubjectID) == "" {
		return fmt.Errorf("%w: subject id required", ErrInvalidArgument)
	}
	if err := r.ResourceScope.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	if r.Version < 0 {
		return fmt.Errorf("%w: request version must be non-negative", ErrInvalidArgument)
	}
	if r.Status == "" {
		return fmt.Errorf("%w: status required", ErrInvalidArgument)
	}
	if r.ExpiresAt != nil && r.ExpiresAt.Before(r.CreatedAt) {
		return fmt.Errorf("%w: expires_at before created_at", ErrInvalidArgument)
	}
	if !r.CreatedAt.IsZero() && !r.UpdatedAt.IsZero() && r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("%w: updated_at before created_at", ErrInvalidArgument)
	}
	if r.CurrentVersionID != "" && strings.TrimSpace(r.CurrentVersionID) == "" {
		return fmt.Errorf("%w: current_version_id invalid", ErrInvalidArgument)
	}
	return nil
}

// ValidateConditionVersion 校验条件版本。
func ValidateConditionVersion(cv ConditionVersion) error {
	if strings.TrimSpace(cv.ID) == "" {
		return fmt.Errorf("%w: condition version id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(cv.RequestID) == "" {
		return fmt.Errorf("%w: condition version request id required", ErrInvalidArgument)
	}
	if err := cv.Scope.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	if cv.VersionNumber < 0 {
		return fmt.Errorf("%w: version number must be non-negative", ErrInvalidArgument)
	}
	if cv.FrozenAt.IsZero() {
		return fmt.Errorf("%w: frozen_at required", ErrInvalidArgument)
	}
	if cv.Hash == "" {
		return fmt.Errorf("%w: hash required", ErrInvalidArgument)
	}
	for i, cond := range cv.Conditions {
		if err := ValidatePurposeCondition(cond); err != nil {
			return fmt.Errorf("condition %d: %w", i, err)
		}
	}
	return nil
}

// ValidatePurposeCondition 校验用途条件。
func ValidatePurposeCondition(p PurposeCondition) error {
	if strings.TrimSpace(p.Description) == "" {
		return fmt.Errorf("%w: purpose description required", ErrInvalidArgument)
	}
	if p.ValidFrom.IsZero() {
		return fmt.Errorf("%w: valid_from required", ErrInvalidArgument)
	}
	if p.ValidUntil.IsZero() {
		return fmt.Errorf("%w: valid_until required", ErrInvalidArgument)
	}
	if !p.ValidUntil.After(p.ValidFrom) {
		return fmt.Errorf("%w: valid_until must be after valid_from", ErrInvalidArgument)
	}
	return nil
}

// ValidateDelegationChain 深度校验委托链。
func ValidateDelegationChain(chain DelegationChain) error {
	if strings.TrimSpace(chain.ID) == "" {
		return fmt.Errorf("%w: chain id required", ErrInvalidArgument)
	}
	if chain.Version < 0 {
		return fmt.Errorf("%w: chain version must be non-negative", ErrInvalidArgument)
	}
	seen := make(map[string]bool)
	for i, node := range chain.Nodes {
		if strings.TrimSpace(node.RequestID) == "" {
			return fmt.Errorf("node %d: request id required", i)
		}
		if seen[node.RequestID] {
			return ErrDelegationCycle
		}
		seen[node.RequestID] = true
		if node.ParentRequestID == node.RequestID {
			return ErrDelegationCycle
		}
		if err := node.AllowedScope.Validate(); err != nil {
			return fmt.Errorf("node %d: %w", i, err)
		}
		if node.CreatedAt.IsZero() {
			return fmt.Errorf("node %d: created_at required", i)
		}
	}
	// 检查父子关系一致性（如果 ParentRequestID 非空，则父节点必须已出现）
	for _, node := range chain.Nodes {
		if node.ParentRequestID != "" && !seen[node.ParentRequestID] {
			return fmt.Errorf("node %s references missing parent %s", node.RequestID, node.ParentRequestID)
		}
	}
	return nil
}

// ValidateReviewRound 校验复核轮次。
func ValidateReviewRound(r ReviewRound) error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: review round id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(r.RequestID) == "" {
		return fmt.Errorf("%w: request id required", ErrInvalidArgument)
	}
	if r.RoundNumber < 0 {
		return fmt.Errorf("%w: round number must be non-negative", ErrInvalidArgument)
	}
	if r.Status != ReviewPending && r.Status != ReviewApproved && r.Status != ReviewRejected {
		return fmt.Errorf("%w: invalid review status", ErrInvalidArgument)
	}
	if r.ReviewedAt != nil && r.ReviewedAt.IsZero() {
		return fmt.Errorf("%w: reviewed_at must be zero or valid time", ErrInvalidArgument)
	}
	return nil
}

// ValidateDecisionCredential 校验决策凭据。
func ValidateDecisionCredential(d DecisionCredential) error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("%w: decision id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(d.RequestID) == "" {
		return fmt.Errorf("%w: request id required", ErrInvalidArgument)
	}
	if d.Type == "" {
		return fmt.Errorf("%w: decision type required", ErrInvalidArgument)
	}
	if d.DecidedAt.IsZero() {
		return fmt.Errorf("%w: decided_at required", ErrInvalidArgument)
	}
	if d.ScopeSnapshot.Type == "" || d.ScopeSnapshot.Identifier == "" {
		return fmt.Errorf("%w: scope snapshot invalid", ErrInvalidArgument)
	}
	if strings.TrimSpace(d.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency key required", ErrInvalidArgument)
	}
	return nil
}

// ValidateTemporarySuspension 校验暂停记录。
func ValidateTemporarySuspension(s TemporarySuspension) error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: suspension id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(s.RequestID) == "" {
		return fmt.Errorf("%w: request id required", ErrInvalidArgument)
	}
	if s.SuspendedAt.IsZero() {
		return fmt.Errorf("%w: suspended_at required", ErrInvalidArgument)
	}
	if s.ResumeAt != nil && s.ResumeAt.Before(s.SuspendedAt) {
		return fmt.Errorf("%w: resume_at before suspended_at", ErrInvalidArgument)
	}
	if s.ResumedAt != nil && s.ResumedAt.Before(s.SuspendedAt) {
		return fmt.Errorf("%w: resumed_at before suspended_at", ErrInvalidArgument)
	}
	return nil
}

// NormalizeResourceScope 返回规范化的资源范围（去除空白）。
func NormalizeResourceScope(scope ResourceScope) ResourceScope {
	scope.Type = strings.TrimSpace(scope.Type)
	scope.Identifier = strings.TrimSpace(scope.Identifier)
	if scope.Properties == nil {
		scope.Properties = make(map[string]string)
	}
	for k, v := range scope.Properties {
		scope.Properties[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return scope
}

// ValidateTimeRange 校验时间区间。
func ValidateTimeRange(start, end time.Time) error {
	if start.IsZero() || end.IsZero() {
		return fmt.Errorf("%w: both start and end are required", ErrInvalidArgument)
	}
	if !end.After(start) {
		return fmt.Errorf("%w: end must be after start", ErrInvalidArgument)
	}
	return nil
}

// ValidateExpirationEvent 校验到期事件。
func ValidateExpirationEvent(e ExpirationEvent) error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("%w: expiration id required", ErrInvalidArgument)
	}
	if strings.TrimSpace(e.RequestID) == "" {
		return fmt.Errorf("%w: request id required", ErrInvalidArgument)
	}
	if e.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expires_at required", ErrInvalidArgument)
	}
	if e.Status != ExpirationPending && e.Status != ExpirationProcessed && e.Status != ExpirationMissed {
		return fmt.Errorf("%w: invalid expiration status", ErrInvalidArgument)
	}
	if e.ProcessedAt != nil && e.ProcessedAt.Before(e.ExpiresAt) {
		return fmt.Errorf("%w: processed_at before expires_at", ErrInvalidArgument)
	}
	return nil
}
