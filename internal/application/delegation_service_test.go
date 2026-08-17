package application

import (
	"context"
	"testing"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

func TestDelegationScopeShrinkEnforced(t *testing.T) {
	ctx := context.Background()
	repo, _ := repository.NewFileRepository(t.TempDir())
	dsvc := NewDelegationService(repo)
	// 创建父申请（启用）和子申请（草稿），以及条件版本
	parent := domain.AuthorizationRequest{ID: "p", Status: domain.StatusEnabled, CurrentVersionID: "cv-p", Version: 1}
	repo.SaveRequest(ctx, parent)
	repo.SaveConditionVersion(ctx, domain.ConditionVersion{ID: "cv-p", Scope: domain.ResourceScope{Type: "doc", Identifier: "d1"}})
	child := domain.AuthorizationRequest{ID: "c", Status: domain.StatusDraft, CurrentVersionID: "cv-c", Version: 1}
	repo.SaveRequest(ctx, child)
	repo.SaveConditionVersion(ctx, domain.ConditionVersion{ID: "cv-c", Scope: domain.ResourceScope{Type: "doc", Identifier: "d1"}})
	if err := dsvc.CreateDelegation(ctx, "p", "c"); err != nil {
		t.Fatal(err)
	}
	// 尝试用更宽范围创建应失败
	child2 := domain.AuthorizationRequest{ID: "c2", Status: domain.StatusDraft, CurrentVersionID: "cv-c2", Version: 1}
	repo.SaveRequest(ctx, child2)
	repo.SaveConditionVersion(ctx, domain.ConditionVersion{ID: "cv-c2", Scope: domain.ResourceScope{Type: "doc", Identifier: "d2"}})
	if err := dsvc.CreateDelegation(ctx, "p", "c2"); err == nil {
		t.Fatal("should fail scope expansion")
	}
}
