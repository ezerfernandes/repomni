package cmd

import (
	"bufio"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/session"
	"github.com/ezerfernandes/repomni/internal/syncer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// documentedCommands returns the full set of command paths that are
// documented in README.md. Each entry is the space-separated path
// from the root command (e.g. "branch create").
func documentedCommands() map[string]bool {
	return map[string]bool{
		"inject":                 true,
		"eject":                  true,
		"status":                 true,
		"list":                   true,
		"exec":                   true,
		"exec diff":              true,
		"branch":                 true,
		"branch create":          true,
		"branch clone":           true,
		"branch list":            true,
		"branch set-state":       true,
		"branch set-ticket":      true,
		"branch set-description": true,
		"branch submit":          true,
		"branch attach":          true,
		"branch checks":          true,
		"branch open":            true,
		"branch ready":           true,
		"branch review":          true,
		"branch merge":           true,
		"branch clean":           true,
		"sync":                   true,
		"sync code":              true,
		"sync state":             true,
		"config":                 true,
		"config global":          true,
		"config repo":            true,
		"config script":          true,
		"session":                true,
		"session list":           true,
		"session show":           true,
		"session search":         true,
		"session export":         true,
		"session resume":         true,
		"session stats":          true,
		"session clean":          true,
	}
}

// documentedFlags returns the expected flags for each command path.
// Only flags documented in the README are listed.
func documentedFlags() map[string][]string {
	return map[string][]string{
		"inject":                 {"all", "dry-run", "force", "copy", "symlink", "json", "yes"},
		"eject":                  {"all", "json"},
		"status":                 {"all", "json", "git", "no-fetch"},
		"list":                   {"names", "json"},
		"exec diff":              {"json", "main-dir", "name-only", "no-sync"},
		"branch create":          {"no-inject", "ticket", "json"},
		"branch clone":           {"no-inject", "ticket", "json"},
		"branch list":            {"state", "json", "detailed"},
		"branch set-state":       {"clear", "json"},
		"branch set-ticket":      {"clear", "json"},
		"branch set-description": {"clear", "json"},
		"branch submit":          {"fill", "draft", "reviewer", "base", "title", "body", "json"},
		"branch attach":          {"current", "json"},
		"branch checks":          {"watch", "json"},
		"branch review":          {"approve", "comment", "json"},
		"branch merge":           {"squash", "rebase", "delete-branch", "json"},
		"branch clean":           {"dry-run", "json", "force", "state"},
		"sync":                   {"dry-run", "autostash", "jobs", "no-fetch", "no-tags", "strategy", "json"},
		"sync code":              {"dry-run", "autostash", "jobs", "no-fetch", "no-tags", "strategy", "json"},
		"sync state":             {"dry-run", "json"},
		"config global":          {"source", "non-interactive", "json"},
		"session list":           {"json", "limit"},
		"session show":           {"json", "limit", "offset", "full"},
		"session search":         {"json", "mode", "limit"},
		"session export":         {"output", "full", "no-tools", "json"},
		"session resume":         {"continue"},
		"session stats":          {"json"},
		"session clean":          {"json", "dry-run", "older-than", "empty", "force"},
	}
}

// collectCommands walks the Cobra command tree and returns a map of
// command paths (e.g. "branch create") to their *cobra.Command.
func collectCommands(root *cobra.Command) map[string]*cobra.Command {
	out := make(map[string]*cobra.Command)
	var walk func(cmd *cobra.Command, prefix string)
	walk = func(cmd *cobra.Command, prefix string) {
		for _, child := range cmd.Commands() {
			path := child.Name()
			if prefix != "" {
				path = prefix + " " + child.Name()
			}
			out[path] = child
			walk(child, path)
		}
	}
	walk(root, "")
	return out
}

// --- Contract tests ---

func TestContractCommandTree(t *testing.T) {
	commands := collectCommands(rootCmd)
	for path := range documentedCommands() {
		if _, ok := commands[path]; !ok {
			t.Errorf("documented command %q not found in Cobra tree", path)
		}
	}
}

func TestContractNoUndocumentedCommands(t *testing.T) {
	documented := documentedCommands()
	commands := collectCommands(rootCmd)
	for path, cmd := range commands {
		// Skip built-in help and completion commands.
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		if !documented[path] {
			t.Errorf("command %q exists in Cobra tree but is not documented", path)
		}
	}
}

func TestContractCommandFlags(t *testing.T) {
	commands := collectCommands(rootCmd)
	for path, expectedFlags := range documentedFlags() {
		cmd, ok := commands[path]
		if !ok {
			t.Errorf("command %q not found in Cobra tree", path)
			continue
		}
		for _, flag := range expectedFlags {
			if cmd.Flags().Lookup(flag) == nil && cmd.InheritedFlags().Lookup(flag) == nil {
				t.Errorf("command %q: documented flag --%s not found", path, flag)
			}
		}
	}
}

func TestContractNoUndocumentedFlags(t *testing.T) {
	documented := documentedFlags()
	commands := collectCommands(rootCmd)
	for path, flags := range documented {
		cmd, ok := commands[path]
		if !ok {
			continue
		}
		docSet := make(map[string]bool)
		for _, f := range flags {
			docSet[f] = true
		}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			// Skip built-in flags.
			if f.Name == "help" || f.Name == "version" {
				return
			}
			if !docSet[f.Name] {
				t.Errorf("command %q: flag --%s exists but is not documented", path, f.Name)
			}
		})
	}

	// Also check persistent flags on session parent (--cli).
	sessionPersistent := map[string]bool{"cli": true}
	if cmd, ok := commands["session"]; ok {
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}
			if !sessionPersistent[f.Name] {
				t.Errorf("command %q: persistent flag --%s exists but is not accounted for", "session", f.Name)
			}
		})
	}
}

func TestContractJSONStructTags(t *testing.T) {
	jsonStructs := []struct {
		name string
		typ  reflect.Type
	}{
		{"syncer.RepoStatus", reflect.TypeOf(syncer.RepoStatus{})},
		{"syncer.SyncResult", reflect.TypeOf(syncer.SyncResult{})},
		{"syncer.SyncSummary", reflect.TypeOf(syncer.SyncSummary{})},
		{"mergestatus.Result", reflect.TypeOf(mergestatus.Result{})},
		{"mergestatus.Summary", reflect.TypeOf(mergestatus.Summary{})},
		{"session.SessionMeta", reflect.TypeOf(session.SessionMeta{})},
		{"session.TokenUsage", reflect.TypeOf(session.TokenUsage{})},
		{"session.Stats", reflect.TypeOf(session.Stats{})},
		{"session.Message", reflect.TypeOf(session.Message{})},
		{"session.ToolUse", reflect.TypeOf(session.ToolUse{})},
		{"session.SearchResult", reflect.TypeOf(session.SearchResult{})},
		{"session.Match", reflect.TypeOf(session.Match{})},
	}

	for _, s := range jsonStructs {
		t.Run(s.name, func(t *testing.T) {
			checkStructTags(t, s.typ, "json", s.name)
		})
	}
}

func TestContractYAMLStructTags(t *testing.T) {
	yamlStructs := []struct {
		name string
		typ  reflect.Type
	}{
		{"config.Config", reflect.TypeOf(config.Config{})},
		{"config.Item", reflect.TypeOf(config.Item{})},
		{"repoconfig.RepoConfig", reflect.TypeOf(repoconfig.RepoConfig{})},
		{"repoconfig.RepoItemConfig", reflect.TypeOf(repoconfig.RepoItemConfig{})},
	}

	for _, s := range yamlStructs {
		t.Run(s.name, func(t *testing.T) {
			checkStructTags(t, s.typ, "yaml", s.name)
		})
	}
}

func checkStructTags(t *testing.T, typ reflect.Type, tagKey, structName string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		// Embedded structs inherit their tags; skip the embed itself.
		if field.Anonymous {
			continue
		}
		tag := field.Tag.Get(tagKey)
		if tag == "" {
			t.Errorf("%s.%s: missing %q struct tag", structName, field.Name, tagKey)
		}
	}
}

func TestContractRequirementsCommandCoverage(t *testing.T) {
	// Map of requirement ID prefixes to the command paths they reference.
	reqToCommand := map[string]string{
		"REQ-INJ":  "inject",
		"REQ-STA":  "status",
		"REQ-EJE":  "eject",
		"REQ-SYC":  "sync code",
		"REQ-SYS":  "sync state",
		"REQ-SYN":  "sync",
		"REQ-CONF": "config global",
		"REQ-CFG":  "config",
	}

	commands := collectCommands(rootCmd)

	reqFile, err := os.Open("../../specs/requirements.md")
	if err != nil {
		t.Fatalf("cannot open specs/requirements.md: %v", err)
	}
	defer reqFile.Close()

	reqPattern := regexp.MustCompile(`^### (REQ-[A-Z]+)-\d+`)
	scanner := bufio.NewScanner(reqFile)
	foundReqs := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		matches := reqPattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		prefix := matches[1]
		foundReqs[prefix] = true

		cmdPath, ok := reqToCommand[prefix]
		if !ok {
			continue
		}

		// For "config" we just need the parent to exist.
		if cmdPath == "config" {
			if _, found := commands["config"]; !found {
				t.Errorf("requirement prefix %s references command %q which is missing", prefix, cmdPath)
			}
			continue
		}

		if _, found := commands[cmdPath]; !found {
			t.Errorf("requirement prefix %s references command %q which is missing from Cobra tree", prefix, cmdPath)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading requirements.md: %v", err)
	}

	// Sanity check: we should have found at least the expected prefixes.
	for prefix := range reqToCommand {
		if !foundReqs[prefix] {
			t.Errorf("expected requirement prefix %s not found in specs/requirements.md", prefix)
		}
	}
}

func TestContractWorkflowStates(t *testing.T) {
	documentedStates := []string{
		"active", "review", "approved", "review-blocked",
		"merged", "closed", "paused",
	}

	knownStates := repoconfig.KnownStates()
	stateSet := make(map[string]bool)
	for _, s := range knownStates {
		stateSet[string(s)] = true
	}

	for _, s := range documentedStates {
		if !stateSet[s] {
			t.Errorf("documented workflow state %q not found in repoconfig.KnownStates()", s)
		}
	}

	docSet := make(map[string]bool)
	for _, s := range documentedStates {
		docSet[s] = true
	}
	for _, s := range knownStates {
		if !docSet[string(s)] {
			t.Errorf("workflow state %q exists in repoconfig.KnownStates() but is not documented", s)
		}
	}
}

func TestContractDefaultItems(t *testing.T) {
	documentedDefaults := []struct {
		targetPath string
		itemType   string
	}{
		{".claude/skills", "directory"},
		{".claude/hooks.json", "file"},
		{".envrc", "file"},
		{".env", "file"},
	}

	defaults := config.DefaultItems()

	if len(defaults) != len(documentedDefaults) {
		t.Fatalf("expected %d default items, got %d", len(documentedDefaults), len(defaults))
	}

	for i, expected := range documentedDefaults {
		actual := defaults[i]
		if actual.TargetPath != expected.targetPath {
			t.Errorf("default item %d: expected target path %q, got %q", i, expected.targetPath, actual.TargetPath)
		}
		if string(actual.Type) != expected.itemType {
			t.Errorf("default item %d: expected type %q, got %q", i, expected.itemType, string(actual.Type))
		}
		if !actual.Enabled {
			t.Errorf("default item %d (%s): expected enabled=true", i, actual.TargetPath)
		}
	}

	// Verify DefaultConfig returns symlink mode.
	cfg := config.DefaultConfig()
	if cfg.Mode != config.ModeSymlink {
		t.Errorf("DefaultConfig mode: expected %q, got %q", config.ModeSymlink, cfg.Mode)
	}
}

func TestContractFlagTypes(t *testing.T) {
	commands := collectCommands(rootCmd)

	boolFlags := map[string][]string{
		"inject":        {"all", "dry-run", "force", "copy", "symlink"},
		"eject":         {"all"},
		"status":        {"all", "json", "git", "no-fetch"},
		"branch submit": {"fill", "draft"},
		"sync":          {"dry-run", "autostash", "no-fetch", "no-tags", "json"},
		"sync code":     {"dry-run", "autostash", "no-fetch", "no-tags", "json"},
		"sync state":    {"dry-run", "json"},
	}

	for path, flags := range boolFlags {
		cmd, ok := commands[path]
		if !ok {
			continue
		}
		for _, flag := range flags {
			f := cmd.Flags().Lookup(flag)
			if f == nil {
				continue
			}
			if f.Value.Type() != "bool" {
				t.Errorf("command %q flag --%s: expected type bool, got %s", path, flag, f.Value.Type())
			}
		}
	}

	stringFlags := map[string][]string{
		"sync":          {"strategy"},
		"sync code":     {"strategy"},
		"branch submit": {"base", "title", "body"},
	}

	for path, flags := range stringFlags {
		cmd, ok := commands[path]
		if !ok {
			continue
		}
		for _, flag := range flags {
			f := cmd.Flags().Lookup(flag)
			if f == nil {
				continue
			}
			if f.Value.Type() != "string" {
				t.Errorf("command %q flag --%s: expected type string, got %s", path, flag, f.Value.Type())
			}
		}
	}

	intFlags := map[string][]string{
		"sync":      {"jobs"},
		"sync code": {"jobs"},
	}

	for path, flags := range intFlags {
		cmd, ok := commands[path]
		if !ok {
			continue
		}
		for _, flag := range flags {
			f := cmd.Flags().Lookup(flag)
			if f == nil {
				continue
			}
			if f.Value.Type() != "int" {
				t.Errorf("command %q flag --%s: expected type int, got %s", path, flag, f.Value.Type())
			}
		}
	}

	// Verify jobs has -j shorthand.
	if cmd, ok := commands["sync code"]; ok {
		f := cmd.Flags().Lookup("jobs")
		if f != nil && f.Shorthand != "j" {
			t.Errorf("sync code --jobs: expected shorthand 'j', got %q", f.Shorthand)
		}
	}
}

func TestContractJSONTagValues(t *testing.T) {
	// Verify specific important JSON tag values match the documented output keys.
	checks := []struct {
		structName string
		typ        reflect.Type
		field      string
		expected   string
	}{
		{"SyncResult", reflect.TypeOf(syncer.SyncResult{}), "Action", "action"},
		{"SyncResult", reflect.TypeOf(syncer.SyncResult{}), "PostDetail", "post_detail"},
		{"SyncSummary", reflect.TypeOf(syncer.SyncSummary{}), "Total", "total"},
		{"SyncSummary", reflect.TypeOf(syncer.SyncSummary{}), "Pulled", "pulled"},
		{"Result", reflect.TypeOf(mergestatus.Result{}), "MergeURL", "merge_url"},
		{"Result", reflect.TypeOf(mergestatus.Result{}), "PreviousState", "previous_state"},
		{"Result", reflect.TypeOf(mergestatus.Result{}), "NewState", "new_state"},
		{"SessionMeta", reflect.TypeOf(session.SessionMeta{}), "SessionID", "session_id"},
		{"SessionMeta", reflect.TypeOf(session.SessionMeta{}), "FirstMessage", "first_message"},
		{"SessionMeta", reflect.TypeOf(session.SessionMeta{}), "DurationSecs", "duration_seconds"},
		{"Stats", reflect.TypeOf(session.Stats{}), "TotalSessions", "total_sessions"},
		{"Stats", reflect.TypeOf(session.Stats{}), "TotalDurationSecs", "total_duration_seconds"},
	}

	for _, c := range checks {
		t.Run(c.structName+"."+c.field, func(t *testing.T) {
			field, ok := c.typ.FieldByName(c.field)
			if !ok {
				t.Fatalf("field %s not found on %s", c.field, c.structName)
			}
			tag := field.Tag.Get("json")
			name := strings.Split(tag, ",")[0]
			if name != c.expected {
				t.Errorf("expected json tag %q, got %q", c.expected, name)
			}
		})
	}
}
