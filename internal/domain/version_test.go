package domain

import "testing"

func TestIncrementVersion(t *testing.T) {
	if IncrementVersion(5) != 6 {
		t.Error("bad increment")
	}
}
