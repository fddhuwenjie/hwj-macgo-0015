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

// TestResumedCanSuspendAgain 校验已恢复的授权可进入新的暂停周期，
// 同时已恢复状态不能直接跳回启用等非法状态。
func TestResumedCanSuspendAgain(t *testing.T) {
	if !CanTransition(StatusResumed, StatusSuspended) {
		t.Fatal("RESUMED -> SUSPENDED should be allowed")
	}
	if !CanTransition(StatusResumed, StatusExpired) {
		t.Fatal("RESUMED -> EXPIRED should still be allowed")
	}
	// 已恢复不应直接回到启用。
	if CanTransition(StatusResumed, StatusEnabled) {
		t.Fatal("RESUMED -> ENABLED should be rejected")
	}

	req := AuthorizationRequest{Status: StatusResumed}
	if err := req.Transition(StatusSuspended); err != nil {
		t.Fatalf("resume->suspend should succeed: %v", err)
	}
	if req.Status != StatusSuspended {
		t.Fatalf("expected SUSPENDED, got %s", req.Status)
	}
}
