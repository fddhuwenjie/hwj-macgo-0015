package repository

import (
	"context"
	"testing"

	"evidence/internal/domain"
)

func TestSaveAndGetSubject(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewFileRepository(t.TempDir())
	s := domain.Subject{ID: "s1", Name: "test"}
	if err := repo.SaveSubject(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSubject(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test" {
		t.Fatal("name mismatch")
	}
}
