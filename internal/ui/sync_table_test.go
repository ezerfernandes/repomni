package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ezer/repoinjector/internal/syncer"
)

func TestSyncActionIcon(t *testing.T) {
	tests := []struct {
		action   string
		expected string
	}{
		{"pulled", "[ok]"},
		{"skipped", "[--]"},
		{"dry-run", "[--]"},
		{"error", "[!!]"},
		{"unknown", "[??]"},
		{"", "[??]"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := syncActionIcon(tt.action)
			if got != tt.expected {
				t.Errorf("syncActionIcon(%q) = %q, want %q", tt.action, got, tt.expected)
			}
		})
	}
}

func TestPrintSyncResults(t *testing.T) {
	results := []syncer.SyncResult{
		{
			RepoStatus: syncer.RepoStatus{
				Name:   "repo-a",
				Branch: "main",
			},
			Action:     "pulled",
			PostDetail: "pulled 3 commits",
		},
		{
			RepoStatus: syncer.RepoStatus{
				Name:   "repo-b",
				Branch: "develop",
			},
			Action:     "skipped",
			PostDetail: "up to date",
		},
	}

	summary := syncer.SyncSummary{
		Total:   2,
		Pulled:  1,
		Current: 1,
	}

	output := captureStdout(t, func() {
		PrintSyncResults(results, summary)
	})

	if !strings.Contains(output, "repo-a") {
		t.Error("expected repo-a in output")
	}
	if !strings.Contains(output, "repo-b") {
		t.Error("expected repo-b in output")
	}
	if !strings.Contains(output, "1 pulled") {
		t.Errorf("expected '1 pulled' in summary, got: %s", output)
	}
	if !strings.Contains(output, "Done.") {
		t.Error("expected 'Done.' in summary")
	}
}

func TestPrintGitStatusTable(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{
			Name:   "my-repo",
			Branch: "main",
			State:  syncer.StateCurrent,
			Dirty:  false,
			Detail: "up to date",
		},
		{
			Name:   "dirty-repo",
			Branch: "feature",
			State:  syncer.StateDirty,
			Dirty:  true,
			Detail: "uncommitted changes",
		},
	}

	output := captureStdout(t, func() {
		PrintGitStatusTable(statuses)
	})

	if !strings.Contains(output, "my-repo") {
		t.Error("expected my-repo in output")
	}
	if !strings.Contains(output, "dirty-repo") {
		t.Error("expected dirty-repo in output")
	}
	if !strings.Contains(output, "Repository") {
		t.Error("expected header row with 'Repository'")
	}
}

func TestPrintSyncJSON(t *testing.T) {
	results := []syncer.SyncResult{
		{
			RepoStatus: syncer.RepoStatus{
				Name:   "repo-a",
				Branch: "main",
				State:  syncer.StatePulled,
			},
			Action: "pulled",
		},
	}
	summary := syncer.SyncSummary{Total: 1, Pulled: 1}

	output := captureStdout(t, func() {
		PrintSyncJSON(results, summary)
	})

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintSyncJSON output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if _, ok := parsed["results"]; !ok {
		t.Error("expected 'results' key in JSON output")
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("expected 'summary' key in JSON output")
	}
}

func TestPrintGitStatusJSON(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{
			Name:   "repo-a",
			Branch: "main",
			State:  syncer.StateCurrent,
		},
	}

	output := captureStdout(t, func() {
		PrintGitStatusJSON(statuses)
	})

	// Verify valid JSON array
	var parsed []interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintGitStatusJSON output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if len(parsed) != 1 {
		t.Errorf("expected 1 entry in JSON array, got %d", len(parsed))
	}
}

func TestPrintSyncResults_Empty(t *testing.T) {
	summary := syncer.SyncSummary{Total: 0}

	output := captureStdout(t, func() {
		PrintSyncResults(nil, summary)
	})

	if !strings.Contains(output, "Repository") {
		t.Error("expected header even with empty results")
	}
	if !strings.Contains(output, "0 pulled") {
		t.Error("expected '0 pulled' in summary")
	}
	if !strings.Contains(output, "Done.") {
		t.Error("expected 'Done.' in summary")
	}
}

func TestPrintSyncResults_AllActions(t *testing.T) {
	results := []syncer.SyncResult{
		{
			RepoStatus: syncer.RepoStatus{Name: "repo-ok", Branch: "main"},
			Action:     "pulled",
			PostDetail: "3 commits",
		},
		{
			RepoStatus: syncer.RepoStatus{Name: "repo-skip", Branch: "main"},
			Action:     "skipped",
			PostDetail: "up to date",
		},
		{
			RepoStatus: syncer.RepoStatus{Name: "repo-err", Branch: "main"},
			Action:     "error",
			PostDetail: "merge conflict",
		},
		{
			RepoStatus: syncer.RepoStatus{Name: "repo-dry", Branch: "main"},
			Action:     "dry-run",
			PostDetail: "would pull",
		},
	}

	summary := syncer.SyncSummary{
		Total:     4,
		Pulled:    1,
		Current:   0,
		Skipped:   2,
		Conflicts: 0,
		Errors:    1,
	}

	output := captureStdout(t, func() {
		PrintSyncResults(results, summary)
	})

	if !strings.Contains(output, "[ok]") {
		t.Error("expected [ok] icon for pulled action")
	}
	if !strings.Contains(output, "[--]") {
		t.Error("expected [--] icon for skipped/dry-run action")
	}
	if !strings.Contains(output, "[!!]") {
		t.Error("expected [!!] icon for error action")
	}
	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected '1 errors' in summary, got: %s", output)
	}
}

func TestPrintSyncResults_SummaryFields(t *testing.T) {
	summary := syncer.SyncSummary{
		Total:     10,
		Pulled:    3,
		Current:   4,
		Skipped:   1,
		Conflicts: 1,
		Errors:    1,
	}

	output := captureStdout(t, func() {
		PrintSyncResults(nil, summary)
	})

	if !strings.Contains(output, "3 pulled") {
		t.Error("expected '3 pulled'")
	}
	if !strings.Contains(output, "4 current") {
		t.Error("expected '4 current'")
	}
	if !strings.Contains(output, "1 skipped") {
		t.Error("expected '1 skipped'")
	}
	if !strings.Contains(output, "1 conflicts") {
		t.Error("expected '1 conflicts'")
	}
	if !strings.Contains(output, "1 errors") {
		t.Error("expected '1 errors'")
	}
	if !strings.Contains(output, "10 repos") {
		t.Error("expected '10 repos'")
	}
}

func TestPrintGitStatusTable_DirtyFlags(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{Name: "clean-repo", Branch: "main", State: syncer.StateCurrent, Dirty: false},
		{Name: "dirty-repo", Branch: "main", State: syncer.StateDirty, Dirty: true},
	}

	output := captureStdout(t, func() {
		PrintGitStatusTable(statuses)
	})

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "clean-repo") {
			if !strings.Contains(line, "No") {
				t.Error("clean repo should show 'No' for dirty")
			}
		}
		if strings.Contains(line, "dirty-repo") {
			if !strings.Contains(line, "Yes") {
				t.Error("dirty repo should show 'Yes' for dirty")
			}
		}
	}
}

func TestPrintGitStatusTable_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		PrintGitStatusTable(nil)
	})

	if !strings.Contains(output, "Repository") {
		t.Error("expected header row even with empty statuses")
	}
	if !strings.Contains(output, "Branch") {
		t.Error("expected Branch header")
	}
}

func TestPrintGitStatusTable_AllStates(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{Name: "r1", Branch: "main", State: syncer.StateCurrent},
		{Name: "r2", Branch: "main", State: syncer.StateBehind, Detail: "2 behind"},
		{Name: "r3", Branch: "main", State: syncer.StateAhead, Detail: "1 ahead"},
		{Name: "r4", Branch: "main", State: syncer.StateDiverged, Detail: "1 ahead, 2 behind"},
		{Name: "r5", Branch: "main", State: syncer.StateDirty, Dirty: true},
		{Name: "r6", Branch: "main", State: syncer.StateNoUpstream},
		{Name: "r7", Branch: "main", State: syncer.StateError, Detail: "fetch failed"},
	}

	output := captureStdout(t, func() {
		PrintGitStatusTable(statuses)
	})

	for _, s := range statuses {
		if !strings.Contains(output, s.Name) {
			t.Errorf("expected %q in output", s.Name)
		}
	}
	if !strings.Contains(output, "current") {
		t.Error("expected 'current' state in output")
	}
	if !strings.Contains(output, "behind") {
		t.Error("expected 'behind' state in output")
	}
	if !strings.Contains(output, "error") {
		t.Error("expected 'error' state in output")
	}
}

func TestPrintGitStatusTable_DetailColumn(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{Name: "r1", Branch: "main", State: syncer.StateBehind, Detail: "3 commits behind origin/main"},
	}

	output := captureStdout(t, func() {
		PrintGitStatusTable(statuses)
	})

	if !strings.Contains(output, "3 commits behind origin/main") {
		t.Error("expected detail text in output")
	}
}

func TestPrintSyncJSON_Empty(t *testing.T) {
	summary := syncer.SyncSummary{Total: 0}

	output := captureStdout(t, func() {
		PrintSyncJSON(nil, summary)
	})

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := parsed["results"]; !ok {
		t.Error("expected 'results' key")
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("expected 'summary' key")
	}
}

func TestPrintSyncJSON_FieldValues(t *testing.T) {
	results := []syncer.SyncResult{
		{
			RepoStatus: syncer.RepoStatus{
				Name:   "my-repo",
				Branch: "develop",
				State:  syncer.StatePulled,
			},
			Action:     "pulled",
			PostDetail: "fast-forward",
		},
	}
	summary := syncer.SyncSummary{Total: 1, Pulled: 1}

	output := captureStdout(t, func() {
		PrintSyncJSON(results, summary)
	})

	var parsed struct {
		Results []struct {
			Name       string `json:"name"`
			Branch     string `json:"branch"`
			Action     string `json:"action"`
			PostDetail string `json:"post_detail"`
		} `json:"results"`
		Summary struct {
			Total  int `json:"total"`
			Pulled int `json:"pulled"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(parsed.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(parsed.Results))
	}
	if parsed.Results[0].Name != "my-repo" {
		t.Errorf("expected name 'my-repo', got %q", parsed.Results[0].Name)
	}
	if parsed.Results[0].Branch != "develop" {
		t.Errorf("expected branch 'develop', got %q", parsed.Results[0].Branch)
	}
	if parsed.Results[0].Action != "pulled" {
		t.Errorf("expected action 'pulled', got %q", parsed.Results[0].Action)
	}
	if parsed.Summary.Total != 1 {
		t.Errorf("expected total 1, got %d", parsed.Summary.Total)
	}
	if parsed.Summary.Pulled != 1 {
		t.Errorf("expected pulled 1, got %d", parsed.Summary.Pulled)
	}
}

func TestPrintGitStatusJSON_FieldValues(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{
			Name:   "test-repo",
			Branch: "feature",
			State:  syncer.StateBehind,
			Dirty:  true,
			Detail: "3 behind",
		},
	}

	output := captureStdout(t, func() {
		PrintGitStatusJSON(statuses)
	})

	var parsed []struct {
		Name   string `json:"name"`
		Branch string `json:"branch"`
		State  string `json:"state"`
		Dirty  bool   `json:"dirty"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(parsed))
	}
	if parsed[0].Name != "test-repo" {
		t.Errorf("expected name 'test-repo', got %q", parsed[0].Name)
	}
	if parsed[0].State != "behind" {
		t.Errorf("expected state 'behind', got %q", parsed[0].State)
	}
	if !parsed[0].Dirty {
		t.Error("expected dirty=true")
	}
	if parsed[0].Detail != "3 behind" {
		t.Errorf("expected detail '3 behind', got %q", parsed[0].Detail)
	}
}

func TestPrintGitStatusJSON_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		PrintGitStatusJSON(nil)
	})

	if !strings.Contains(output, "null") {
		t.Errorf("expected 'null' for nil slice, got: %s", output)
	}
}

func TestPrintSyncResults_ConflictsInSummary(t *testing.T) {
	summary := syncer.SyncSummary{
		Total:     5,
		Pulled:    1,
		Current:   1,
		Skipped:   1,
		Conflicts: 2,
		Errors:    0,
	}

	output := captureStdout(t, func() {
		PrintSyncResults(nil, summary)
	})

	if !strings.Contains(output, "2 conflicts") {
		t.Errorf("expected '2 conflicts', got: %s", output)
	}
}

func TestPrintSyncResults_OnlyPulled(t *testing.T) {
	results := []syncer.SyncResult{
		{
			RepoStatus: syncer.RepoStatus{Name: "repo-1", Branch: "main"},
			Action:     "pulled",
			PostDetail: "1 commit",
		},
	}
	summary := syncer.SyncSummary{Total: 1, Pulled: 1}

	output := captureStdout(t, func() {
		PrintSyncResults(results, summary)
	})

	if !strings.Contains(output, "1 pulled") {
		t.Error("expected '1 pulled'")
	}
	if !strings.Contains(output, "[ok]") {
		t.Error("expected [ok] icon")
	}
}

func TestPrintGitStatusTable_NoUpstream(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{Name: "orphan", Branch: "feature", State: syncer.StateNoUpstream, Detail: "no tracking branch"},
	}

	output := captureStdout(t, func() {
		PrintGitStatusTable(statuses)
	})

	if !strings.Contains(output, "orphan") {
		t.Error("expected repo name in output")
	}
	if !strings.Contains(output, "no-upstream") {
		t.Error("expected 'no-upstream' state in output")
	}
}

func TestPrintGitStatusJSON_Multiple(t *testing.T) {
	statuses := []syncer.RepoStatus{
		{Name: "repo-1", Branch: "main", State: syncer.StateCurrent},
		{Name: "repo-2", Branch: "develop", State: syncer.StateBehind},
		{Name: "repo-3", Branch: "feature", State: syncer.StateAhead},
	}

	output := captureStdout(t, func() {
		PrintGitStatusJSON(statuses)
	})

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(parsed) != 3 {
		t.Errorf("expected 3 entries, got %d", len(parsed))
	}
}
