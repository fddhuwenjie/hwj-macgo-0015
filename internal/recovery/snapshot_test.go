package recovery

import (
	"context"
	"testing"
)

func TestSnapshotCreateRestore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sm := NewSnapshotManager(dir)
	if err := sm.CreateSnapshot(ctx, "snap1"); err != nil {
		t.Fatal(err)
	}
	if err := sm.RestoreFromSnapshot(ctx, "snap1"); err != nil {
		t.Fatal(err)
	}
}
