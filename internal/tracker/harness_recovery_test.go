//go:build cgo

package tracker

import (
	"context"
	"testing"

	"github.com/steveyegge/beads/internal/testutil"
	"github.com/steveyegge/beads/internal/types"
)

// TestNewTestStoreRecoversFromContainerLoss guards the shared-container
// harness against the CI flake where the Dolt container drops every
// connection mid-suite ("[mysql] packets.go:58 unexpected EOF", then
// "invalid connection" on every later test). Killing the container here must
// cost nothing beyond this test: the next newTestStore call has to notice the
// dead server, replace it, and hand back a working store.
//
// The kill removes the container (a crashed one would linger as exited), so this
// covers "server is gone", not "server alive but old connections dropped"; the
// latter needs no replacement because a fresh connection succeeds.
func TestNewTestStoreRecoversFromContainerLoss(t *testing.T) {
	testutil.RequireDoltBinary(t)
	if testServerPort == 0 || testSharedDB == "" {
		t.Skip("shared test Dolt database not initialized, skipping test")
	}

	if err := testutil.KillDoltContainer(); err != nil {
		t.Fatalf("KillDoltContainer() error: %v", err)
	}
	if testutil.DoltServerReachable() {
		t.Fatal("Dolt server still reachable after the container was killed")
	}

	store := newTestStore(t)

	ctx := context.Background()
	issue := &types.Issue{
		ID:        "bd-recovered",
		Title:     "Created after container loss",
		Status:    types.StatusOpen,
		IssueType: types.TypeTask,
		Priority:  2,
	}
	if err := store.CreateIssue(ctx, issue, "test-actor"); err != nil {
		t.Fatalf("CreateIssue() after recovery error: %v", err)
	}
	got, err := store.GetIssue(ctx, "bd-recovered")
	if err != nil {
		t.Fatalf("GetIssue() after recovery error: %v", err)
	}
	if got.Title != issue.Title {
		t.Fatalf("title = %q, want %q", got.Title, issue.Title)
	}
}
