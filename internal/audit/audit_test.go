package audit

import "testing"

func TestAuditTrailIntegrity(t *testing.T) {
	trail := NewAuditTrail()
	trail.Append("a", "create", "r1", "details")
	trail.Append("b", "approve", "r1", "details")
	if !trail.Verify() {
		t.Error("audit trail verification failed")
	}
	// 篡改
	entries := trail.Entries()
	entries[0].Details = "tampered"
	// 直接修改内部是不可能的，因为返回拷贝；但验证仍然基于内部，所以通过
	if !trail.Verify() {
		t.Error("should still verify after copy tamper")
	}
}
