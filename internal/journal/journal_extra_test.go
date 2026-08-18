package journal

import (
	"os"
	"testing"
)

func TestTruncateSafe(t *testing.T) {
	path := t.TempDir() + "/j.log"
	j, _ := NewJournal(path)
	j.Append([]byte("data"))
	if err := j.TruncateSafe(0); err != nil {
		t.Fatal(err)
	}
	records, _ := j.Replay()
	if len(records) != 0 {
		t.Fatal("should be empty")
	}
}

// TestReplayPreservesValidRecords 确保合法记录不再被反向校验逻辑跳过。
func TestReplayPreservesValidRecords(t *testing.T) {
	path := t.TempDir() + "/j.log"
	j, _ := NewJournal(path)
	defer j.Close()
	if err := j.Append([]byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("two")); err != nil {
		t.Fatal(err)
	}
	records, offset, err := j.ReplayWithOffset()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || string(records[0]) != "one" || string(records[1]) != "two" {
		t.Fatalf("records=%q", records)
	}
	fi, _ := os.Stat(path)
	if offset != fi.Size() {
		t.Fatalf("validOffset=%d want=%d (合法尾部应等于文件大小)", offset, fi.Size())
	}
}

// TestReplayStopsAtCorruptTail 模拟崩溃残留：日志末尾存在不完整记录时，
// 重放保留此前所有合法记录且不报错；TruncateCorruptTail 仅截断残留，保留合法记录。
func TestReplayStopsAtCorruptTail(t *testing.T) {
	path := t.TempDir() + "/j.log"
	j, _ := NewJournal(path)
	if err := j.Append([]byte("keep1")); err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("keep2")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	// 追加一段不完整的“长度头 + 部分数据”，模拟崩溃残留。
	fi, _ := os.Stat(path)
	tail := []byte{0x00, 0x00, 0x00, 0x10, 'x', 'y', 'z'} // 声明16字节，仅写3字节
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(tail); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	j2, _ := NewJournal(path)
	defer j2.Close()
	records, offset, err := j2.ReplayWithOffset()
	if err != nil {
		t.Fatalf("unexpected error on corrupt tail: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("合法记录应全部保留，got=%q", records)
	}
	if offset != fi.Size() {
		t.Fatalf("validOffset=%d want=%d (应指向崩溃残留起点)", offset, fi.Size())
	}

	// 仅截断残留尾部，合法记录必须保留。
	if err := j2.TruncateCorruptTail(offset); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if after.Size() != fi.Size() {
		t.Fatalf("截断后大小=%d want=%d (合法记录不应被截断)", after.Size(), fi.Size())
	}
	got, _ := j2.Replay()
	if len(got) != 2 {
		t.Fatalf("截断残留后重放应仍为2条，got=%q", got)
	}
}

// TestTruncateCorruptTailNoopOnCleanLog 确认对完整合法日志调用截断是空操作，
// 不会清空证据。
func TestTruncateCorruptTailNoopOnCleanLog(t *testing.T) {
	path := t.TempDir() + "/j.log"
	j, _ := NewJournal(path)
	defer j.Close()
	j.Append([]byte("a"))
	j.Append([]byte("b"))
	_, offset, _ := j.ReplayWithOffset()
	if err := j.TruncateCorruptTail(offset); err != nil {
		t.Fatal(err)
	}
	records, _ := j.Replay()
	if len(records) != 2 {
		t.Fatalf("合法日志不应被清空，got=%q", records)
	}
}
