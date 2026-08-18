package application_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"evidence/internal/application"
	"evidence/internal/domain"
	"evidence/internal/repository"
)

var errInjectedConditionWrite = errors.New("injected condition version failure")

type failingConditionRepository struct {
	domain.Repository
	written chan struct{}
	release chan struct{}
}

func (r *failingConditionRepository) BeginTx(ctx context.Context) (domain.Transaction, error) {
	tx, err := r.Repository.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	return &failingConditionTransaction{Transaction: tx, written: r.written, release: r.release}, nil
}

type failingConditionTransaction struct {
	domain.Transaction
	written chan struct{}
	release chan struct{}
}

func (t *failingConditionTransaction) Repository() domain.Repository {
	return &failingConditionTxRepository{
		Repository: t.Transaction.Repository(),
		written:    t.written,
		release:    t.release,
	}
}

type failingConditionTxRepository struct {
	domain.Repository
	written chan struct{}
	release chan struct{}
}

func (r *failingConditionTxRepository) SaveConditionVersion(ctx context.Context, cv domain.ConditionVersion) error {
	if err := r.Repository.SaveConditionVersion(ctx, cv); err != nil {
		return err
	}
	close(r.written)
	select {
	case <-r.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return errInjectedConditionWrite
}

func TestBug03ConcurrentSubmitKeepsFrozenVersionAtomic(t *testing.T) {
	t.Run("concurrent submissions have one complete winner", func(t *testing.T) {
		ctx := context.Background()
		repo, err := repository.NewFileRepository(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		from := time.Now().UTC()
		conditions := []domain.PurposeCondition{{Description: "read", ValidFrom: from, ValidUntil: from.Add(time.Hour)}}
		scope := domain.ResourceScope{Type: "document", Identifier: "contract"}

		for round := 0; round < 6; round++ {
			requestID := fmt.Sprintf("request-%02d", round)
			if err := repo.SaveRequest(ctx, domain.AuthorizationRequest{ID: requestID, Status: domain.StatusDraft, Version: 1}); err != nil {
				t.Fatal(err)
			}
			services := []*application.AuthorizationService{
				application.NewAuthorizationService(repo),
				application.NewAuthorizationService(repo),
			}
			start := make(chan struct{})
			var ready sync.WaitGroup
			var done sync.WaitGroup
			results := make(chan error, len(services))
			ready.Add(len(services))
			done.Add(len(services))
			for _, service := range services {
				service := service
				go func() {
					defer done.Done()
					ready.Done()
					<-start
					results <- service.SubmitForReview(ctx, requestID, scope, conditions)
				}()
			}
			ready.Wait()
			close(start)
			done.Wait()
			close(results)

			successes := 0
			conflicts := 0
			for err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, domain.ErrInvalidStateTransition):
					conflicts++
				default:
					t.Fatalf("round %d unexpected concurrent submit error: %v", round, err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("round %d successes=%d conflicts=%d, want 1 and 1", round, successes, conflicts)
			}
			request, err := repo.GetRequest(ctx, requestID)
			if err != nil {
				t.Fatal(err)
			}
			version, err := repo.GetConditionVersion(ctx, request.CurrentVersionID)
			if err != nil {
				t.Fatalf("round %d frozen version %q is unreachable: %v", round, request.CurrentVersionID, err)
			}
			if request.Status != domain.StatusConditionFrozen || version.RequestID != requestID {
				t.Fatalf("round %d split state: request=%#v version=%#v", round, request, version)
			}
		}
	})

	t.Run("failed transaction is never visible to a concurrent reader", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		base, err := repository.NewFileRepository(root)
		if err != nil {
			t.Fatal(err)
		}
		const requestID = "request-rollback"
		if err := base.SaveRequest(ctx, domain.AuthorizationRequest{ID: requestID, Status: domain.StatusDraft, Version: 1}); err != nil {
			t.Fatal(err)
		}
		written := make(chan struct{})
		release := make(chan struct{})
		wrapped := &failingConditionRepository{Repository: base, written: written, release: release}
		service := application.NewAuthorizationService(wrapped)
		from := time.Now().UTC()
		result := make(chan error, 1)
		go func() {
			result <- service.SubmitForReview(ctx, requestID,
				domain.ResourceScope{Type: "document", Identifier: "rollback"},
				[]domain.PurposeCondition{{Description: "audit", ValidFrom: from, ValidUntil: from.Add(time.Hour)}})
		}()

		<-written
		visible, err := base.GetRequest(ctx, requestID)
		if err != nil {
			t.Fatal(err)
		}
		if visible.Status != domain.StatusDraft || visible.CurrentVersionID != "" || visible.Version != 1 {
			t.Fatalf("concurrent reader observed uncommitted request state: %#v", visible)
		}
		close(release)
		if err := <-result; !errors.Is(err, errInjectedConditionWrite) {
			t.Fatalf("submit error=%v, want injected condition failure", err)
		}
		after, err := base.GetRequest(ctx, requestID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Status != domain.StatusDraft || after.CurrentVersionID != "" || after.Version != 1 {
			t.Fatalf("rollback left partial request state: %#v", after)
		}
		entries, err := os.ReadDir(filepath.Join(root, "condition_versions"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("rollback left %d condition version files", len(entries))
		}
	})
}
