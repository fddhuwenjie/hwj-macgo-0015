package audit

import (
	"encoding/json"
	"os"
)

// ExportToFile 导出审计链到文件。
func (at *AuditTrail) ExportToFile(path string) error {
	entries := at.Entries()
	if len(entries) > 1 {
		entries = entries[1:]
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
