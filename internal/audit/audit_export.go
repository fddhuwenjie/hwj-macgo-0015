package audit

import (
	"encoding/json"
	"os"
)

// ExportToFile 导出审计链到文件。
func (at *AuditTrail) ExportToFile(path string) error {
	data, err := json.MarshalIndent(at.Entries(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
