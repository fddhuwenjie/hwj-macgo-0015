package query

import (
	"testing"

	"evidence/internal/domain"
)

func TestFindConflicts(t *testing.T) {
	reqs := []domain.AuthorizationRequest{
		{ID: "r1", ResourceScope: domain.ResourceScope{Type: "doc", Identifier: "d1"}},
		{ID: "r2", ResourceScope: domain.ResourceScope{Type: "doc", Identifier: "d2"}},
	}
	res := FindConflicts(reqs, domain.ResourceScope{Type: "doc", Identifier: "d1"})
	if len(res) != 1 || res[0].ID != "r1" {
		t.Fatal("conflict wrong")
	}
}
