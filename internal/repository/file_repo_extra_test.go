package repository

import (
	"context"
	"testing"
)

func TestCheckIntegrity(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())
	if err := repo.CheckIntegrity(ctx); err != nil {
		t.Fatal(err)
	}
}
