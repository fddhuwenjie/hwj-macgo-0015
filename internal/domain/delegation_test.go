package domain

import "testing"

func TestDelegationCycle(t *testing.T) {
	chain := DelegationChain{Nodes: []DelegationNode{
		{RequestID: "r1", ParentRequestID: "r2"},
		{RequestID: "r2", ParentRequestID: "r1"},
	}}
	if err := chain.ValidateNoCycle(); err == nil {
		t.Error("should detect cycle")
	}
}
