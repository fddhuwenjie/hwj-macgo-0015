package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"evidence/internal/domain"
)

// 提供一些额外方法以满足接口完整性或辅助功能。

// CountRequests 返回申请总数。
func (r *FileRepository) CountRequests(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(r.rootDir, "requests"))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count, nil
}

// CheckIntegrity 简单校验所有JSON文件可解析。
func (r *FileRepository) CheckIntegrity(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	subdirs := []string{"subjects", "requests", "condition_versions", "review_rounds", "decisions", "suspensions", "delegation_chains", "expirations"}
	for _, sd := range subdirs {
		dir := filepath.Join(r.rootDir, sd)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var v interface{}
			if err := json.Unmarshal(data, &v); err != nil {
				return fmt.Errorf("invalid json %s: %w", path, err)
			}
		}
	}
	return nil
}

// 实现一些未在接口中但可能用到的辅助
var _ domain.Repository = (*FileRepository)(nil)
