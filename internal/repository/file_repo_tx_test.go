package repository

import (
	"context"
	"testing"

	"evidence/internal/domain"
)

// TestTransactionCommitPersistsBothWrites 校验事务提交后，两条写入一起落盘。
func TestTransactionCommitPersistsBothWrites(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())

	tx, _ := repo.BeginTx(ctx)
	defer tx.Rollback(ctx)
	r := tx.Repository()

	if err := r.SaveRequest(ctx, domain.AuthorizationRequest{ID: "r", Status: domain.StatusEnabled, Version: 1}); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveSuspension(ctx, domain.TemporarySuspension{ID: "s", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	// 提交前磁盘上不应有任何写入。
	if n, _ := repo.CountRequests(ctx); n != 0 {
		t.Fatalf("pre-commit: expected 0 requests on disk, got %d", n)
	}
	if n, _ := repo.CountSuspensions(ctx); n != 0 {
		t.Fatalf("pre-commit: expected 0 suspensions on disk, got %d", n)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// 提交后两条写入一起可见。
	if n, _ := repo.CountRequests(ctx); n != 1 {
		t.Fatalf("post-commit: expected 1 request on disk, got %d", n)
	}
	if n, _ := repo.CountSuspensions(ctx); n != 1 {
		t.Fatalf("post-commit: expected 1 suspension on disk, got %d", n)
	}
}

// TestTransactionRollbackDiscardsWrites 校验回滚丢弃全部缓冲写入，不留下部分更新。
func TestTransactionRollbackDiscardsWrites(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())

	// 先落盘一条基线申请。
	if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{ID: "r", Status: domain.StatusEnabled, Version: 1}); err != nil {
		t.Fatal(err)
	}

	tx, _ := repo.BeginTx(ctx)
	r := tx.Repository()
	// 在事务内覆盖申请并新增暂停记录。
	if err := r.SaveRequest(ctx, domain.AuthorizationRequest{ID: "r", Status: domain.StatusSuspended, Version: 2}); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveSuspension(ctx, domain.TemporarySuspension{ID: "s", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// 回滚后：暂停记录不存在，申请仍是提交前的旧状态。
	if n, _ := repo.CountSuspensions(ctx); n != 0 {
		t.Fatalf("post-rollback: expected 0 suspensions, got %d (partial update leaked)", n)
	}
	got, _ := repo.GetRequest(ctx, "r")
	if got.Status != domain.StatusEnabled || got.Version != 1 {
		t.Fatalf("post-rollback: expected ENABLED/v1, got %s/v%d", got.Status, got.Version)
	}
}

// TestTransactionReadYourWrites 校验事务内可读到本事务尚未提交的写入。
func TestTransactionReadYourWrites(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())

	tx, _ := repo.BeginTx(ctx)
	defer tx.Rollback(ctx)
	r := tx.Repository()

	if err := r.SaveRequest(ctx, domain.AuthorizationRequest{ID: "r", Status: domain.StatusSuspended, Version: 4}); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetRequest(ctx, "r")
	if err != nil {
		t.Fatalf("read-your-writes: %v", err)
	}
	if got.Status != domain.StatusSuspended || got.Version != 4 {
		t.Fatalf("read-your-writes: got %s/v%d", got.Status, got.Version)
	}
	// 落盘数据仍为空（未提交）。
	if n, _ := repo.CountRequests(ctx); n != 0 {
		t.Fatalf("uncommitted write should not be on disk, got %d", n)
	}
}

// TestTransactionListRequestsMergesBuffer 校验事务内列表查询与未提交写入一致。
func TestTransactionListRequestsMergesBuffer(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())
	// 落盘一条启用申请。
	if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{ID: "a", SubjectID: "s", Status: domain.StatusEnabled}); err != nil {
		t.Fatal(err)
	}

	tx, _ := repo.BeginTx(ctx)
	defer tx.Rollback(ctx)
	r := tx.Repository()
	// 事务内把 a 改为暂停，并新增 b（恢复）。
	if err := r.SaveRequest(ctx, domain.AuthorizationRequest{ID: "a", SubjectID: "s", Status: domain.StatusSuspended}); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveRequest(ctx, domain.AuthorizationRequest{ID: "b", SubjectID: "s", Status: domain.StatusResumed}); err != nil {
		t.Fatal(err)
	}
	// 列表查询应反映缓冲：a 为暂停、b 为恢复，共 2 条。
	all, err := r.ListRequests(ctx, domain.RequestFilter{SubjectID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(all))
	}
	// 按状态过滤应仅命中 b。
	resumed, err := r.ListRequests(ctx, domain.RequestFilter{SubjectID: "s", Status: domain.StatusResumed})
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 1 || resumed[0].ID != "b" {
		t.Fatalf("expected only b resumed, got %#v", resumed)
	}
}
