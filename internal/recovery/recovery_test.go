package recovery

import (
	"context"
	"testing"
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
