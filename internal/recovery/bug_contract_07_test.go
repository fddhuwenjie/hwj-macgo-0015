package recovery_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"evidence/internal/journal"
	"evidence/internal/recovery"
)

func TestBug07TruncatedTailKeepsPrefixAndAllowsRetry(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "journal.log")

	j, err := journal.NewJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("committed-prefix")); err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("partially-written")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(journalPath, info.Size()-3); err != nil {
		t.Fatal(err)
	}

	if _, err := recovery.Recover(context.Background(), filepath.Join(root, "repo"), journalPath); err != nil {
		t.Fatalf("recovery should preserve a valid prefix: %v", err)
	}

	j, err = journal.NewJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	records, err := j.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || string(records[0]) != "committed-prefix" {
		t.Fatalf("records after recovery = %#v, want the committed prefix", records)
	}
	if err := j.Append([]byte("retry-after-recovery")); err != nil {
		t.Fatal(err)
	}
	records, err = j.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || string(records[1]) != "retry-after-recovery" {
		t.Fatalf("retry append was not durable: %#v", records)
	}
}
