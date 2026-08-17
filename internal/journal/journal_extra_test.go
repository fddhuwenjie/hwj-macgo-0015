package journal

import "testing"

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
