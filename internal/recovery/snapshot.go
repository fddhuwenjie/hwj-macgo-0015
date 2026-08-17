package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"evidence/internal/repository"
)

// SnapshotManager 管理原子快照。
type SnapshotManager struct {
	repoDir string
}

// NewSnapshotManager 创建快照管理器。
func NewSnapshotManager(repoDir string) *SnapshotManager {
	return &SnapshotManager{repoDir: repoDir}
}

// CreateSnapshot 创建当前状态的快照文件。
func (sm *SnapshotManager) CreateSnapshot(ctx context.Context, name string) error {
	// 复制整个仓库目录到快照目录（简化：打包元数据）
	// 实际可遍历所有JSON文件并合并为一个快照JSON
	// 这里简单创建快照索引
	snapDir := filepath.Join(sm.repoDir, "snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}
	data := map[string]interface{}{
		"name": name,
		"time": ctx.Value("time"),
	}
	b, _ := json.Marshal(data)
	path := filepath.Join(snapDir, name+".json")
	return os.WriteFile(path, b, 0644)
}

// RestoreFromSnapshot 从快照恢复（简化，实际需完整实现）。
func (sm *SnapshotManager) RestoreFromSnapshot(ctx context.Context, name string) error {
	path := filepath.Join(sm.repoDir, "snapshots", name+".json")
	if _, err := os.Stat(path); err != nil {
		return err
	}
	// 恢复逻辑留空，由repository配合实现
	return nil
}

// 辅助
var _ = repository.NewFileRepository
var _ = fmt.Sprintf
