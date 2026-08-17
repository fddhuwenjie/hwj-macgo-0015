package recovery

import (
	"context"
	"os"
	"path/filepath"

	"evidence/internal/journal"
	"evidence/internal/repository"
)

// Recover 从日志和快照恢复仓库。
func Recover(ctx context.Context, repoDir string, journalPath string) (*repository.FileRepository, error) {
	j, err := journal.NewJournal(journalPath)
	if err != nil {
		return nil, err
	}
	defer j.Close()
	records, err := j.Replay()
	if err != nil {
		return nil, err
	}
	// 先建立仓库目录
	repo, err := repository.NewFileRepository(repoDir)
	if err != nil {
		return nil, err
	}
	// 重放记录（简化：记录为命令，需实现命令解析；此处占位）
	_ = records
	// 检查完整性
	if err := repo.CheckIntegrity(ctx); err != nil {
		return nil, err
	}
	return repo, nil
}

// CleanupTmpFiles 清理临时文件。
func CleanupTmpFiles(repoDir string) error {
	return filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".tmp" {
			return os.Remove(path)
		}
		return nil
	})
}
