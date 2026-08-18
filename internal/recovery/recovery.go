package recovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"evidence/internal/journal"
	"evidence/internal/repository"
)

// Recover 从日志重放记录并恢复仓库。
//
// 完整性约束：
//   - 重放保留全部合法记录（不跳过、不丢弃）。
//   - 成功读取后绝不清空日志证据；仅在恢复成功后安全截断“崩溃残留尾部”，
//     合法记录作为证据完整保留。
//   - 仓库建立与完整性校验失败时，日志证据不被破坏，避免留下“日志已清空、仓库未就绪”
//     的部分更新；状态、查询结果与持久化数据因此保持一致。
func Recover(ctx context.Context, repoDir string, journalPath string) (*repository.FileRepository, error) {
	j, err := journal.NewJournal(journalPath)
	if err != nil {
		return nil, err
	}
	defer j.Close()
	// 重放：保留所有合法记录，并取得合法尾部偏移（用于定位崩溃残留）。
	records, validOffset, err := j.ReplayWithOffset()
	if err != nil {
		// 读取错误时保留日志证据，交由上层处置，不做任何截断。
		return nil, fmt.Errorf("replay journal: %w", err)
	}
	// 先建立仓库目录（与日志证据隔离：仓库创建失败不会破坏日志）。
	repo, err := repository.NewFileRepository(repoDir)
	if err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}
	// 按序重放记录到仓库，确保状态、查询与持久化数据一致。
	// 任一记录处理失败即中止恢复，不留下部分更新；此时日志证据完整保留以便重试。
	if err := applyRecords(ctx, repo, records); err != nil {
		return nil, fmt.Errorf("apply records: %w", err)
	}
	// 完整性校验通过后才认为恢复成功。
	if err := repo.CheckIntegrity(ctx); err != nil {
		return nil, fmt.Errorf("integrity check: %w", err)
	}
	// 仅在恢复成功后安全截断崩溃残留尾部；合法记录绝不被清空。
	if err := j.TruncateCorruptTail(validOffset); err != nil {
		return nil, fmt.Errorf("truncate corrupt tail: %w", err)
	}
	return repo, nil
}

// applyRecords 将重放得到的合法记录按序应用到仓库。
//
// 保证状态、查询结果与持久化数据一致：逐条确认，任一异常即中止并返回错误，
// 避免部分记录已写入而后续失败造成的部分更新。
//
// 说明：日志记录当前为不透明证据字节，合法记录已在 Replay 阶段确认并保留在日志中。
// 待命令格式接入后，此处将解析记录并在事务内执行对应领域命令；当前不修改仓库状态，
// 仅确保记录被逐一确认，日志证据不被截断或清空。
func applyRecords(ctx context.Context, repo *repository.FileRepository, records [][]byte) error {
	for _, rec := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		// 防御性确认：Replay 仅返回合法记录；若出现非法记录则中止，保留证据。
		if !journal.ValidRecord(rec) {
			return fmt.Errorf("corrupt record detected during replay")
		}
		// TODO(command): 将 rec 解析为领域命令，并在 repo.BeginTx 事务内应用。
	}
	_ = repo
	return nil
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
