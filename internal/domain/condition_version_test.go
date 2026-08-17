package domain

import "testing"

func TestConditionVersionHashChanges(t *testing.T) {
	cv := ConditionVersion{
		Scope: ResourceScope{Type: "doc", Identifier: "d1"},
		Conditions: []PurposeCondition{{Description: "read"}},
	}
	h1 := cv.ComputeHash()
	cv.Conditions[0].Description = "write"
	h2 := cv.ComputeHash()
	if h1 == h2 {
		t.Error("hash should change")
	}
}
