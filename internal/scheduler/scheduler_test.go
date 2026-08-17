package scheduler

import (
	"context"
	"testing"
	"time"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

func TestSchedulerProcessesExpiration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo, _ := repository.NewFileRepository(t.TempDir())
	// 插入一个启用且已过期的申请
	past := time.Now().Add(-1 * time.Hour)
	req := domain.AuthorizationRequest{
		ID:        "r1",
		Status:    domain.StatusEnabled,
		ExpiresAt: &past,
		Version:   1,
	}
	repo.SaveRequest(ctx, req)
	sched := NewScheduler(repo)
	sched.Start(ctx, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	sched.Stop()
	got, _ := repo.GetRequest(ctx, "r1")
	if got.Status != domain.StatusExpired {
		t.Fatalf("expected expired, got %s", got.Status)
	}
}
