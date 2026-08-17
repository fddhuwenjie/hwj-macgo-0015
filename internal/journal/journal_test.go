package journal

import "testing"

func TestAppendReplay(t *testing.T) {
	path := t.TempDir() + "/journal.log"
	j, err := NewJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	data := []byte("hello")
	if err := j.Append(data); err != nil {
		t.Fatal(err)
	}
	records, err := j.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || string(records[0]) != "hello" {
		t.Fatal("replay mismatch")
	}
}
