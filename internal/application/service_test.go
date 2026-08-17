package application

import (
	"context"
	"testing"
	"time"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

func TestSubmitReviewEnableChain(t *testing.T) {
	ctx := context.Background()
	repo, _ := repository.NewFileRepository(t.TempDir())
	svc := NewAuthorizationService(repo)
	// 创建主体和草稿申请
	subj := domain.Subject{ID: "s1"}
	repo.SaveSubject(ctx, subj)
	req := domain.AuthorizationRequest{
		ID:            "r1",
		SubjectID:     "s1",
		ResourceScope: domain.ResourceScope{Type: "doc", Identifier: "d1"},
		Status:        domain.StatusDraft,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExpiresAt:     timePtr(time.Now().Add(24 * time.Hour)),
	}
	repo.SaveRequest(ctx, req)
	// 提交复核
	scope := domain.ResourceScope{Type: "doc", Identifier: "d1"}
	conds := []domain.PurposeCondition{{Description: "read", ValidFrom: time.Now(), ValidUntil: time.Now().Add(48 * time.Hour)}}
	if err := svc.SubmitForReview(ctx, "r1", scope, conds); err != nil {
		t.Fatal(err)
	}
	// 复核通过
	if err := svc.Review(ctx, "r1", "reviewer1", true, "ok"); err != nil {
		t.Fatal(err)
	}
	// 启用
	if err := svc.Enable(ctx, "r1", "admin"); err != nil {
		t.Fatal(err)
	}
	// 验证状态
	got, _ := repo.GetRequest(ctx, "r1")
	if got.Status != domain.StatusEnabled {
		t.Fatalf("expected enabled, got %s", got.Status)
	}
}
