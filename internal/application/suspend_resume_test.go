package application

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

// savedSuspensionCount 统计当前已落盘的暂停记录数量。
func savedSuspensionCount(t *testing.T, repo *repository.FileRepository) int {
	t.Helper()
	n, err := repo.CountSuspensions(context.Background())
	if err != nil {
		t.Fatalf("count suspensions: %v", err)
	}
	return n
}

// TestSuspendRejectsNonActiveStates 校验草稿、条件冻结、复核中、已到期、已撤回
// 的申请均被拒绝直接暂停，且被拒绝时不留下任何暂停记录（无部分更新）。
func TestSuspendRejectsNonActiveStates(t *testing.T) {
	nonActive := []domain.AuthorizationStatus{
		domain.StatusDraft,
		domain.StatusConditionFrozen,
		domain.StatusUnderReview,
		domain.StatusExpired,
		domain.StatusWithdrawn,
	}
	for _, st := range nonActive {
		st := st
		t.Run(string(st), func(t *testing.T) {
			ctx := context.Background()
			repo, _ := repository.NewFileRepository(filepath.Join(t.TempDir(), "store"))
			svc := NewAuthorizationService(repo)
			if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{
				ID:      "r",
				SubjectID: "s",
				Status:  st,
				Version: 1,
			}); err != nil {
				t.Fatal(err)
			}
			err := svc.Suspend(ctx, "r", "review", nil)
			if err == nil {
				t.Fatalf("%s should be rejected from suspending", st)
			}
			if got := savedSuspensionCount(t, repo); got != 0 {
				t.Fatalf("%s rejected but left %d suspension record(s): no partial update allowed", st, got)
			}
			// 申请状态与版本不变。
			req, _ := repo.GetRequest(ctx, "r")
			if req.Status != st {
				t.Fatalf("status mutated on rejection: got %s want %s", req.Status, st)
			}
			if req.Version != 1 {
				t.Fatalf("version mutated on rejection: got %d want 1", req.Version)
			}
		})
	}
}

// TestSuspendAllowsEnabledAndResumed 校验启用与恢复均可发起暂停。
func TestSuspendAllowsEnabledAndResumed(t *testing.T) {
	for _, st := range []domain.AuthorizationStatus{domain.StatusEnabled, domain.StatusResumed} {
		st := st
		t.Run(string(st), func(t *testing.T) {
			ctx := context.Background()
			repo, _ := repository.NewFileRepository(filepath.Join(t.TempDir(), "store"))
			svc := NewAuthorizationService(repo)
			if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{
				ID:      "r",
				SubjectID: "s",
				Status:  st,
				Version: 1,
			}); err != nil {
				t.Fatal(err)
			}
			if err := svc.Suspend(ctx, "r", "review", nil); err != nil {
				t.Fatalf("%s -> suspend should succeed: %v", st, err)
			}
			req, _ := repo.GetRequest(ctx, "r")
			if req.Status != domain.StatusSuspended {
				t.Fatalf("expected SUSPENDED, got %s", req.Status)
			}
			if req.Version != 2 {
				t.Fatalf("expected version 2, got %d", req.Version)
			}
		})
	}
}

// TestSuspendResumeCycle 校验完整的"暂停—恢复—再次暂停"周期。
func TestSuspendResumeCycle(t *testing.T) {
	ctx := context.Background()
	repo, _ := repository.NewFileRepository(filepath.Join(t.TempDir(), "store"))
	svc := NewAuthorizationService(repo)

	exp := time.Now().Add(24 * time.Hour)
	if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{
		ID:        "r",
		SubjectID: "s",
		Status:    domain.StatusEnabled,
		Version:   1,
		ExpiresAt: &exp,
	}); err != nil {
		t.Fatal(err)
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	assertStatus := func(want domain.AuthorizationStatus, wantVer int64) {
		t.Helper()
		req, _ := repo.GetRequest(ctx, "r")
		if req.Status != want {
			t.Fatalf("status: got %s want %s", req.Status, want)
		}
		if req.Version != wantVer {
			t.Fatalf("version: got %d want %d", req.Version, wantVer)
		}
	}

	// 第一轮暂停与恢复。
	must(svc.Suspend(ctx, "r", "first", nil))
	assertStatus(domain.StatusSuspended, 2)
	must(svc.Resume(ctx, "r"))
	assertStatus(domain.StatusResumed, 3)

	// 恢复后再次进入新的暂停周期。
	must(svc.Suspend(ctx, "r", "second", nil))
	assertStatus(domain.StatusSuspended, 4)
	must(svc.Resume(ctx, "r"))
	assertStatus(domain.StatusResumed, 5)

	// 恢复后仍可到期。
	req, _ := repo.GetRequest(ctx, "r")
	if err := req.Transition(domain.StatusExpired); err != nil {
		t.Fatalf("resumed->expired should be allowed: %v", err)
	}
}
