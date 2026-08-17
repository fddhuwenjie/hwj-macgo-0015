package audit

import (
	"os"
	"testing"
)

func TestExportToFile(t *testing.T) {
	trail := NewAuditTrail()
	trail.Append("a", "act", "r1", "det")
	path := t.TempDir() + "/audit.json"
	if err := trail.ExportToFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file missing")
	}
}
