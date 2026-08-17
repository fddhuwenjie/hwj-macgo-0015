package domain

import "testing"
import "time"

func TestNowUTC(t *testing.T) {
	n := Now()
	if n.Location() != time.UTC {
		t.Error("not UTC")
	}
}
