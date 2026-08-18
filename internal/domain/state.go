package domain

import "fmt"

// ValidTransitions 定义状态转移合法表。
var ValidTransitions = map[AuthorizationStatus][]AuthorizationStatus{
	StatusDraft:           {StatusConditionFrozen, StatusWithdrawn},
	StatusConditionFrozen: {StatusUnderReview, StatusWithdrawn},
	StatusUnderReview:     {StatusEnabled, StatusWithdrawn},
	StatusEnabled:         {StatusSuspended, StatusExpired, StatusWithdrawn},
	StatusSuspended:       {StatusResumed, StatusExpired, StatusUnderReview},
	StatusResumed:         {StatusSuspended, StatusExpired},
	StatusExpired:         {},
	StatusWithdrawn:       {},
}

// CanTransition 判断状态转移是否合法。
func CanTransition(from, to AuthorizationStatus) bool {
	for _, t := range ValidTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Transition 执行状态转移，返回错误若非法。
func (a *AuthorizationRequest) Transition(to AuthorizationStatus) error {
	if !CanTransition(a.Status, to) {
		return fmt.Errorf("%w: from %s to %s", ErrInvalidStateTransition, a.Status, to)
	}
	a.Status = to
	return nil
}
