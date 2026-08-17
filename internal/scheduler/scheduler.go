package scheduler

import (
	"context"
	"time"

	"evidence/internal/application"
	"evidence/internal/domain"
)

// Scheduler 后台任务调度器，定期处理到期。
type Scheduler struct {
	expSvc *application.ExpirationService
	stop   chan struct{}
}

// NewScheduler 创建调度器。
func NewScheduler(repo domain.Repository) *Scheduler {
	return &Scheduler{
		expSvc: application.NewExpirationService(repo),
		stop:   make(chan struct{}),
	}
}

// Start 启动后台循环。
func (s *Scheduler) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.expSvc.ProcessDueExpirations(ctx, time.Now())
			case <-s.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop 停止调度器。
func (s *Scheduler) Stop() {
	close(s.stop)
}
