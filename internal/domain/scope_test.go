package domain

import "testing"

func TestResourceScopeContains(t *testing.T) {
	a := ResourceScope{Type: "doc", Identifier: "1", Properties: map[string]string{"tenant": "t1"}}
	b := ResourceScope{Type: "doc", Identifier: "1", Properties: map[string]string{"tenant": "t1"}}
	if !a.Contains(b) {
		t.Error("should contain")
	}
	b.Properties["tenant"] = "t2"
	if a.Contains(b) {
		t.Error("should not contain different prop")
	}
}
