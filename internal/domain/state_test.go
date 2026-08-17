package domain

import "testing"

func TestStateTransitions(t *testing.T) {
	req := AuthorizationRequest{Status: StatusDraft}
	if err := req.Transition(StatusConditionFrozen); err != nil {
		t.Fatal(err)
	}
	if req.Status != StatusConditionFrozen {
		t.Fatal("transition failed")
	}
	if err := req.Transition(StatusEnabled); err == nil {
		t.Fatal("illegal transition should fail")
	}
}
