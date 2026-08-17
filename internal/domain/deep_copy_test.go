package domain

import (
	"testing"
	"time"
)

func TestAuthorizationRequestClone(t *testing.T) {
	now := time.Now()
	a := AuthorizationRequest{
		ID:                "r1",
		ResourceScope:     ResourceScope{Type: "doc", Properties: map[string]string{"k": "v"}},
		PurposeConditions: []PurposeCondition{{Description: "use"}},
		ExpiresAt:         &now,
	}
	b := a.Clone()
	b.ResourceScope.Properties["k"] = "changed"
	if a.ResourceScope.Properties["k"] != "v" {
		t.Error("original mutated")
	}
	b.PurposeConditions[0].Description = "changed"
	if a.PurposeConditions[0].Description != "use" {
		t.Error("slice mutated")
	}
}
