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

// TestReplayTornPayloadTruncatesTailOnly covers a crash that left the final
// record's payload half-written: the committed prefix must survive, the torn
// record is dropped, and appending resumes right after the prefix.
func TestReplayTornPayloadTruncatesTailOnly(t *testing.T) {
	path := t.TempDir() + "/torn_payload.log"
	j, _ := NewJournal(path)
	if err := j.Append([]byte("committed-prefix")); err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("partially-written")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if err := os.Truncate(path, info.Size()-3); err != nil {
		t.Fatal(err)
	}

	j, _ = NewJournal(path)
	records, err := j.Replay()
	if err != nil {
		t.Fatalf("torn tail must self-heal, got %v", err)
	}
	if len(records) != 1 || string(records[0]) != "committed-prefix" {
		t.Fatalf("prefix lost: %#v", records)
	}
	if err := j.Append([]byte("retry-after-recovery")); err != nil {
		t.Fatal(err)
	}
	records, _ = j.Replay()
	if len(records) != 2 || string(records[1]) != "retry-after-recovery" {
		t.Fatalf("retry not durable over a healed tail: %#v", records)
	}
	j.Close()

	// On-disk size must equal exactly the two complete records, proving the
	// torn bytes were physically removed rather than merely skipped.
	info, _ = os.Stat(path)
	wantSize := int64(4+len("committed-prefix")) + int64(4+len("retry-after-recovery"))
	if info.Size() != wantSize {
		t.Fatalf("file size = %d, want %d (no torn residue)", info.Size(), wantSize)
	}
}

// TestReplayTornHeaderTruncatesTailOnly covers a crash that left only part of
// the final record's 4-byte length header on disk.
func TestReplayTornHeaderTruncatesTailOnly(t *testing.T) {
	path := t.TempDir() + "/torn_header.log"
	j, _ := NewJournal(path)
	if err := j.Append([]byte("committed-prefix")); err != nil {
		t.Fatal(err)
	}
	if err := j.Append([]byte("next-record")); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	// Trim into the length header of the last record (leave only 1 of 4 bytes).
	cut := int64(4 + len("next-record") - 1) // keep first header byte of last record
	if err := os.Truncate(path, info.Size()-cut); err != nil {
		t.Fatal(err)
	}

	j, _ = NewJournal(path)
	records, err := j.Replay()
	if err != nil {
		t.Fatalf("torn header must self-heal, got %v", err)
	}
	if len(records) != 1 || string(records[0]) != "committed-prefix" {
		t.Fatalf("prefix lost: %#v", records)
	}
	if err := j.Append([]byte("retry-after-recovery")); err != nil {
		t.Fatal(err)
	}
	records, _ = j.Replay()
	if len(records) != 2 || string(records[1]) != "retry-after-recovery" {
		t.Fatalf("retry not durable over a healed tail: %#v", records)
	}
	j.Close()
}

// TestReplayIdempotentAcrossReopen ensures a healed journal stays clean: a
// second open+replay with no further truncation returns the same prefix.
func TestReplayIdempotentAcrossReopen(t *testing.T) {
	path := t.TempDir() + "/idempotent.log"
	j, _ := NewJournal(path)
	j.Append([]byte("committed-prefix"))
	j.Append([]byte("partially-written"))
	j.Close()
	info, _ := os.Stat(path)
	os.Truncate(path, info.Size()-3)

	j, _ = NewJournal(path)
	if _, err := j.Replay(); err != nil {
		t.Fatalf("first replay: %v", err)
	}
	j.Close()

	j, _ = NewJournal(path)
	defer j.Close()
	records, err := j.Replay()
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if len(records) != 1 || string(records[0]) != "committed-prefix" {
		t.Fatalf("prefix drifted on reopen: %#v", records)
	}
}
