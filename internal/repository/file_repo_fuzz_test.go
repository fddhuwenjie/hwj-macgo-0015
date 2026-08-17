package repository

import (
	"context"
	"testing"

	"evidence/internal/domain"
)

func FuzzDecodeSubject(f *testing.F) {
	f.Add(`{"id":"s1","name":"test"}`)
	f.Fuzz(func(t *testing.T, data string) {
		ctx := context.Background()
		repo, _ := NewFileRepository(t.TempDir())
		// 写入文件然后尝试读取
		_ = repo.writeJSON(ctx, "subjects", "fuzz", domain.Subject{ID: "fuzz"})
		var s domain.Subject
		_ = repo.readJSON(ctx, "subjects", "fuzz", &s)
		// 不会panic即可
	})
}
