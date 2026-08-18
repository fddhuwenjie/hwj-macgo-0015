package recovery

import (
	"context"
	"os"
	"testing"

	"evidence/internal/journal"
)

func TestRecoverEmpty(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := Recover(ctx, dir, dir+"/journal.log")
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("nil repo")
	}
}

// TestRecoverPreservesValidJournalOnFailure 确认恢复流程在任何后续步骤失败时，
// 都不会清空日志证据（避免部分更新）。
func TestRecoverPreservesValidJournalOnFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := dir + "/journal.log"

	j, _ := journal.NewJournal(path)
	_ = j.Append([]byte("one"))
	_ = j.Append([]byte("two"))
	_ = j.Close()

	// 仓库目录不可创建（用一个已存在文件占位，使 MkdirAll 失败），
	// 恢复必须在破坏日志前返回错误，且日志证据完整保留。
	badRepo := dir + "/repo"
	if err := os.WriteFile(badRepo, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(ctx, badRepo, path); err == nil {
		t.Fatal("expected error when repo dir cannot be created")
	}

	// 日志必须仍包含两条合法记录。
	reopened, _ := journal.NewJournal(path)
	defer reopened.Close()
	records, err := reopened.Replay()
	if err != nil || len(records) != 2 {
		t.Fatalf("日志证据应被保留: records=%q err=%v", records, err)
	}
}

// TestRecoverTruncatesCorruptTail 确认恢复会截断崩溃残留尾部，同时保留合法记录。
func TestRecoverTruncatesCorruptTail(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := dir + "/journal.log"

	j, _ := journal.NewJournal(path)
	_ = j.Append([]byte("keep1"))
	_ = j.Append([]byte("keep2"))
	_ = j.Close()

	fi, _ := os.Stat(path)
	// 追加不完整记录模拟崩溃残留。
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte{0x00, 0x00, 0x00, 0x10, 'x', 'y', 'z'})
	f.Close()

	if _, err := Recover(ctx, dir+"/repo", path); err != nil {
		t.Fatal(err)
	}

	// 残留尾部应被截断，合法记录保留，日志长度回到崩溃前。
	after, _ := os.Stat(path)
	if after.Size() != fi.Size() {
		t.Fatalf("残留应被截断，size=%d want=%d", after.Size(), fi.Size())
	}
	reopened, _ := journal.NewJournal(path)
	defer reopened.Close()
	records, err := reopened.Replay()
	if err != nil || len(records) != 2 {
		t.Fatalf("合法记录应保留: records=%q err=%v", records, err)
	}
}
