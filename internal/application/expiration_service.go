package application

import (
	"context"
	"time"

	"evidence/internal/domain"
)

// ExpirationService 处理到期事件。
type ExpirationService struct {
	repo domain.Repository
}

// NewExpirationService 创建到期服务。
func NewExpirationService(repo domain.Repository) *ExpirationService {
	return &ExpirationService{repo: repo}
}

// ProcessDueExpirations 处理所有已到期但未处理的申请。
func (e *ExpirationService) ProcessDueExpirations(ctx context.Context, now time.Time) error {
	tx, err := e.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repo := tx.Repository()
	// 列出启用/恢复且过期时间已到的申请
	reqs, err := repo.ListRequests(ctx, domain.RequestFilter{
		Status: domain.StatusEnabled,
	})
	if err != nil {
		return err
	}
	resumed, err := repo.ListRequests(ctx, domain.RequestFilter{
		Status: domain.StatusResumed,
	})
	if err != nil {
		return err
	}
	reqs = append(reqs, resumed...)
	for _, req := range reqs {
		if req.ExpiresAt != nil && req.ExpiresAt.Before(now) {
			evt := domain.ExpirationEvent{
				ID:        "exp-" + req.ID + "-" + now.Format("20060102150405"),
				RequestID: req.ID,
				ExpiresAt: *req.ExpiresAt,
				Status:    domain.ExpirationPending,
			}
			if err := repo.SaveExpiration(ctx, evt); err != nil {
				return err
			}
			if err := req.Transition(domain.StatusExpired); err != nil {
				return err
			}
			req.Version++
			if err := repo.SaveRequest(ctx, req); err != nil {
				return err
			}
			// 可记录处理时间，但简化
		}
	}
	return tx.Commit(ctx)
}
