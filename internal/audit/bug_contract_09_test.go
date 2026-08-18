package audit

import (
	"encoding/json"
	"os"
	"testing"

	"evidence/internal/domain"
)

func TestBug09AuditExportKeepsCompleteChain(t *testing.T) {
	trail := NewAuditTrail()
	trail.Append("actor-a", "create", "request", "created")
	trail.Append("actor-b", "approve", "request", "approved")
	path := t.TempDir() + "/audit.json"
	if err := trail.ExportToFile(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []domain.AuditEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Hash == "" || entries[1].Hash == "" || entries[1].PrevHash != entries[0].Hash {
		t.Fatalf("exported chain is incomplete: %#v", entries)
	}
}
