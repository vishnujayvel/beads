package main

import "testing"

// TestBuildListFilterAssigneeIncludesWisps guards against ocb-3txj: `bd list
// --assignee=X` silently hid ephemeral wisps/molecules assigned to X because
// buildListFilter defaulted filter.SkipWisps=true for any non-infra issue
// type, with no carve-out for an explicit --assignee filter. That made an
// assigned wisp (e.g. a witness's own patrol molecule) invisible to `bd list
// --assignee=<self>` even though `bd query "assignee=..."` found it fine,
// breaking any reconciliation loop that used `bd list --assignee` to dedupe
// its own wisps.
func TestBuildListFilterAssigneeIncludesWisps(t *testing.T) {
	cfg := listFilterConfig{}

	t.Run("assignee set skips the SkipWisps default", func(t *testing.T) {
		for _, issueType := range []string{"", "molecule", "task"} {
			in := listInput{assignee: "beads/gastown.witness", issueType: issueType}
			filter, err := buildListFilter(in, cfg)
			if err != nil {
				t.Fatalf("buildListFilter(%q): %v", issueType, err)
			}
			if filter.SkipWisps {
				t.Errorf("issueType=%q: SkipWisps = true, want false when --assignee is set (assigned wisps must stay visible)", issueType)
			}
			if filter.Assignee == nil || *filter.Assignee != "beads/gastown.witness" {
				t.Errorf("issueType=%q: Assignee = %v, want beads/gastown.witness", issueType, filter.Assignee)
			}
		}
	})

	t.Run("no assignee keeps today's pool-noise default", func(t *testing.T) {
		for _, issueType := range []string{"", "molecule", "task"} {
			in := listInput{issueType: issueType}
			filter, err := buildListFilter(in, cfg)
			if err != nil {
				t.Fatalf("buildListFilter(%q): %v", issueType, err)
			}
			if !filter.SkipWisps {
				t.Errorf("issueType=%q: SkipWisps = false, want true (default `bd list` still hides unassigned pool/wisp noise)", issueType)
			}
		}
	})

	t.Run("--include-infra still wins regardless of assignee", func(t *testing.T) {
		in := listInput{assignee: "beads/gastown.witness", includeInfra: true}
		filter, err := buildListFilter(in, cfg)
		if err != nil {
			t.Fatalf("buildListFilter: %v", err)
		}
		if filter.SkipWisps {
			t.Errorf("SkipWisps = true, want false when --include-infra is passed")
		}
	})

	t.Run("infra issue type without assignee still merges wisps", func(t *testing.T) {
		// cfg.isInfra("message") is true by default (domain.DefaultInfraTypes),
		// so this path already worked before the fix -- pin it so the new
		// assignee carve-out doesn't accidentally narrow it.
		in := listInput{issueType: "message"}
		filter, err := buildListFilter(in, cfg)
		if err != nil {
			t.Fatalf("buildListFilter: %v", err)
		}
		if filter.SkipWisps {
			t.Errorf("SkipWisps = true, want false for infra issue type %q", in.issueType)
		}
	})
}
