package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"evidence/internal/domain"
)

// AuthorizationService 授权应用服务，协调领域逻辑与持久化。
type AuthorizationService struct {
	repo domain.Repository
	mu   sync.Mutex
}

// NewAuthorizationService 创建服务。
func NewAuthorizationService(repo domain.Repository) *AuthorizationService {
	return &AuthorizationService{repo: repo}
}

// SelfCheck 执行基本自检。
func (s *AuthorizationService) SelfCheck(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 创建主体
	subj := domain.Subject{ID: "self", Name: "selfcheck"}
	if err := s.repo.SaveSubject(ctx, subj); err != nil {
		return fmt.Errorf("save subject: %w", err)
	}
	// 创建申请草稿
	req := domain.AuthorizationRequest{
		ID:            "req-self",
		SubjectID:     subj.ID,
		ResourceScope: domain.ResourceScope{Type: "document", Identifier: "doc1"},
		Status:        domain.StatusDraft,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.repo.SaveRequest(ctx, req); err != nil {
		return fmt.Errorf("save request: %w", err)
	}
	return nil
}

// SubmitForReview 提交申请进入冻结条件与复核。
func (s *AuthorizationService) SubmitForReview(ctx context.Context, requestID string, scope domain.ResourceScope, conds []domain.PurposeCondition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != domain.StatusDraft {
		return domain.ErrInvalidStateTransition
	}
	// 创建条件版本
	cv := domain.ConditionVersion{
		ID:        fmt.Sprintf("cv-%s-%d", requestID, time.Now().UnixNano()),
		RequestID: requestID,
		Scope:     scope,
		Conditions: conds,
		FrozenAt:  time.Now(),
	}
	cv.Hash = cv.ComputeHash()
	// 先持久化被引用的条件版本，再更新引用它的申请；两者在同一事务缓冲内，
	// 由 Commit 原子发布。若任一步失败，Rollback 丢弃全部缓冲，磁盘不留半更新。
	if err := repo.SaveConditionVersion(ctx, cv); err != nil {
		return err
	}
	req.Status = domain.StatusConditionFrozen
	req.CurrentVersionID = cv.ID
	req.UpdatedAt = time.Now()
	req.Version++
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Review 执行一次复核。
func (s *AuthorizationService) Review(ctx context.Context, requestID string, reviewerID string, approve bool, comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != domain.StatusConditionFrozen && req.Status != domain.StatusUnderReview {
		return domain.ErrInvalidStateTransition
	}
	round := domain.ReviewRound{
		ID:          fmt.Sprintf("rr-%s-%d", requestID, time.Now().UnixNano()),
		RequestID:   requestID,
		ReviewerID:  reviewerID,
		Status:      domain.ReviewRejected,
		Comment:     comment,
		ReviewedAt:  timePtr(time.Now()),
	}
	if approve {
		round.Status = domain.ReviewApproved
		// 复核通过后冻结条件版本
		cv, err := repo.GetConditionVersion(ctx, req.CurrentVersionID)
		if err != nil {
			return err
		}
		round.DecisionVersion = cv.ID
		// 移动到 UnderReview (继续多轮) 或直接启用由后续决策控制
		// 这里简单：一轮通过则进入 UnderReview 状态等待最终启用决策
		req.Status = domain.StatusUnderReview
	} else {
		// 驳回回到 Draft 或保持
		req.Status = domain.StatusDraft
	}
	req.Version++
	if err := repo.SaveReviewRound(ctx, round); err != nil {
		return err
	}
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Enable 启用授权，执行范围收窄、委托链无环与到期窗口校验。
func (s *AuthorizationService) Enable(ctx context.Context, requestID string, decidedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != domain.StatusUnderReview {
		return domain.ErrInvalidStateTransition
	}
	// 获取当前条件版本
	cv, err := repo.GetConditionVersion(ctx, req.CurrentVersionID)
	if err != nil {
		return err
	}
	// 校验范围收窄：如果存在委托链，上游范围必须包含当前范围，且不能扩大
	if req.DelegationChainID != "" {
		chain, err := repo.GetDelegationChain(ctx, req.DelegationChainID)
		if err != nil {
			return err
		}
		if err := chain.ValidateNoCycle(); err != nil {
			return err
		}
		// 找到父申请的范围，确保不扩大
		for _, node := range chain.Nodes {
			if node.RequestID == requestID && node.ParentRequestID != "" {
				parent, err := repo.GetRequest(ctx, node.ParentRequestID)
				if err != nil {
					return err
				}
				parentCV, err := repo.GetConditionVersion(ctx, parent.CurrentVersionID)
				if err != nil {
					return err
				}
				if !parentCV.Scope.Contains(cv.Scope) {
					return domain.ErrScopeExpansion
				}
			}
		}
	}
	// 到期时间必须非空且在未来
	if req.ExpiresAt == nil || !req.ExpiresAt.After(time.Now()) {
		return domain.ErrInvalidArgument
	}
	// 创建决策凭据
	cred := domain.DecisionCredential{
		ID:               fmt.Sprintf("dc-%s-%d", requestID, time.Now().UnixNano()),
		RequestID:        requestID,
		Type:             domain.DecisionEnable,
		ConditionVersion: cv.ID,
		ScopeSnapshot:    cv.Scope,
		DecidedAt:        time.Now(),
		DecidedBy:        decidedBy,
		IdempotencyKey:   fmt.Sprintf("enable-%s-%d", requestID, req.Version),
	}
	if err := repo.SaveDecision(ctx, cred); err != nil {
		return err
	}
	// 更新状态
	if err := req.Transition(domain.StatusEnabled); err != nil {
		return err
	}
	req.Version++
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Suspend 暂停授权。
func (s *AuthorizationService) Suspend(ctx context.Context, requestID string, reason string, resumeAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != domain.StatusEnabled && req.Status != domain.StatusResumed {
		return domain.ErrInvalidStateTransition
	}
	susp := domain.TemporarySuspension{
		ID:          fmt.Sprintf("sus-%s-%d", requestID, time.Now().UnixNano()),
		RequestID:   requestID,
		Reason:      reason,
		SuspendedAt: time.Now(),
		ResumeAt:    resumeAt,
	}
	if err := repo.SaveSuspension(ctx, susp); err != nil {
		return err
	}
	if err := req.Transition(domain.StatusSuspended); err != nil {
		return err
	}
	req.Version++
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Resume 恢复授权。
func (s *AuthorizationService) Resume(ctx context.Context, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != domain.StatusSuspended {
		return domain.ErrInvalidStateTransition
	}
	if err := req.Transition(domain.StatusResumed); err != nil {
		return err
	}
	req.Version++
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Withdraw 撤回申请。
func (s *AuthorizationService) Withdraw(ctx context.Context, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	req, err := repo.GetRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status == domain.StatusExpired || req.Status == domain.StatusWithdrawn {
		return domain.ErrInvalidStateTransition
	}
	if err := req.Transition(domain.StatusWithdrawn); err != nil {
		return err
	}
	req.Version++
	if err := repo.SaveRequest(ctx, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// timePtr helper
func timePtr(t time.Time) *time.Time { return &t }
