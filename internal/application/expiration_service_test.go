package application

import (
	"context"
	"testing"
	"time"

	"evidence/internal/domain"
	"evidence/internal/repository"
)

func TestProcessDueExpirations(t *testing.T) {
	ctx := context.Background()
	repo, _ := repository.NewFileRepository(t.TempDir())
	expSvc := NewExpirationService(repo)
	past := time.Now().Add(-1 * time.Hour)
	req := domain.AuthorizationRequest{ID: "r1", Status: domain.StatusEnabled, ExpiresAt: &past, Version: 1}
	repo.SaveRequest(ctx, req)
	if err := expSvc.ProcessDueExpirations(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetRequest(ctx, "r1")
	if got.Status != domain.StatusExpired {
		t.Fatal("not expired")
	}
}
