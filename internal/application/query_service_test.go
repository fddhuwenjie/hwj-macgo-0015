package application

import (
	"context"
	"testing"
	"time"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

func TestEffectiveRequestsAtAndRiskSort(t *testing.T) {
	ctx := context.Background()
	repo, _ := repository.NewFileRepository(t.TempDir())
	qs := NewQueryService(repo)
	// 创建两个启用申请，不同到期时间
	now := time.Now()
	r1 := domain.AuthorizationRequest{ID: "r1", Status: domain.StatusEnabled, ExpiresAt: timePtr(now.Add(1 * time.Hour)), ResourceScope: domain.ResourceScope{Type: "doc", Identifier: "d1"}}
	r2 := domain.AuthorizationRequest{ID: "r2", Status: domain.StatusEnabled, ExpiresAt: timePtr(now.Add(2 * time.Hour)), ResourceScope: domain.ResourceScope{Type: "doc", Identifier: "d2"}}
	repo.SaveRequest(ctx, r1)
	repo.SaveRequest(ctx, r2)
	// 指定时间点有效
	reqs, err := qs.EffectiveRequestsAt(ctx, now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2, got %d", len(reqs))
	}
	// 风险排序
	risk, err := qs.RequestsByExpirationRisk(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if risk[0].ID != "r1" {
		t.Fatalf("expected r1 first")
	}
}
