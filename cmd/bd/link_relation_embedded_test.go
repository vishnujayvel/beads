//go:build cgo

package main

import (
	"os"
	"strings"
	"testing"
)

// TestEmbeddedLinkRelationPhrase is the GH#5542 regression for `bd link` confirmation
// text: a non-blocks --type must not print "depends on" (sibling of #5529 for `bd dep add`).
// Requires BEADS_TEST_EMBEDDED_DOLT=1.
func TestEmbeddedLinkRelationPhrase(t *testing.T) {
	if os.Getenv("BEADS_TEST_EMBEDDED_DOLT") != "1" {
		t.Skip("set BEADS_TEST_EMBEDDED_DOLT=1 to run embedded dolt integration tests")
	}

	bd := buildEmbeddedBD(t)
	dir, _, _ := bdInit(t, bd, "--prefix", "lk")

	from := bdCreate(t, bd, dir, "Link from")
	to := bdCreate(t, bd, dir, "Link to")

	related := bdLinkForHierarchyTest(t, bd, dir, from.ID, to.ID, "--type", "related")
	if !strings.Contains(related, "Linked") {
		t.Fatalf("expected Linked confirmation, got: %s", related)
	}
	if !strings.Contains(related, "is related to") {
		t.Errorf("related link must use relation phrase, got: %s", related)
	}
	if strings.Contains(related, "depends on") {
		t.Errorf("related link must not claim depends on, got: %s", related)
	}

	// Default blocks path stays pinned to "depends on".
	blocked := bdCreate(t, bd, dir, "Blocked")
	blocker := bdCreate(t, bd, dir, "Blocker")
	blocks := bdLinkForHierarchyTest(t, bd, dir, blocked.ID, blocker.ID)
	if !strings.Contains(blocks, "depends on") {
		t.Errorf("default blocks link must still say depends on, got: %s", blocks)
	}
}
