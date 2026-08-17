package query

import (
	"testing"
	"time"

	"evidence/internal/domain"
)

func TestSortByRiskStable(t *testing.T) {
	t1 := time.Now().Add(1 * time.Hour)
	t2 := time.Now().Add(2 * time.Hour)
	reqs := []domain.AuthorizationRequest{
		{ID: "a", ExpiresAt: &t2},
		{ID: "b", ExpiresAt: &t1},
		{ID: "c", ExpiresAt: nil},
	}
	SortByRiskStable(reqs)
	if reqs[0].ID != "b" || reqs[1].ID != "a" || reqs[2].ID != "c" {
		t.Fatal("order incorrect")
	}
}
