// Package session discovers, parses, and analyzes Claude Code session files.
// It reads JSONL session data from ~/.claude/projects/ and provides structured
// access to session metadata, messages, and token usage.
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TokenUsage aggregates token counts across all messages in a session.
type TokenUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
}

// SessionMeta holds summary metadata about a single Claude Code session.
type SessionMeta struct {
	SessionID    string     `json:"session_id"`
	ProjectPath  string     `json:"project_path"`
	FilePath     string     `json:"file_path"`
	FirstMessage string     `json:"first_message"`
	MessageCount int        `json:"message_count"`
	CreatedAt    time.Time  `json:"created_at"`
	ModifiedAt   time.Time  `json:"modified_at"`
	SizeBytes    int64      `json:"size_bytes"`
	DurationSecs float64    `json:"duration_seconds"`
	Tokens       TokenUsage `json:"tokens"`
}

// Message represents a single parsed message from a session.
type Message struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	UUID      string    `json:"uuid"`
	ToolUses  []ToolUse `json:"tool_uses,omitempty"`
}

// ToolUse represents a tool invocation within an assistant message.
type ToolUse struct {
	Name string `json:"name"`
}

// SearchResult holds a session match from a search operation.
type SearchResult struct {
	Meta    SessionMeta `json:"session"`
	Matches []Match     `json:"matches"`
}

// Match represents a single content match within a session.
type Match struct {
	Type    string `json:"type"`
	Preview string `json:"preview"`
	Index   int    `json:"index"`
}

// EncodePath converts an absolute filesystem path to the Claude Code
// encoded directory name by replacing each '/' with '-'.
func EncodePath(absPath string) string {
	return strings.ReplaceAll(absPath, "/", "-")
}

// ClaudeProjectsDir returns the path to ~/.claude/projects/.
func ClaudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// ProjectSessionDir returns the full path to the session directory
// for a given project path.
func ProjectSessionDir(projectPath string) (string, error) {
	base, err := ClaudeProjectsDir()
	if err != nil {
		return "", err
	}
	encoded := EncodePath(projectPath)
	return filepath.Join(base, encoded), nil
}

// Discover finds all non-empty .jsonl session files for the given
// project path. Returns SessionMeta for each, sorted by ModifiedAt
// descending (newest first). Zero-byte files are filtered out.
func Discover(projectPath string) ([]SessionMeta, error) {
	return DiscoverWithLimit(projectPath, 0)
}

// DiscoverWithLimit is like Discover but returns at most limit sessions.
// If limit is 0, all sessions are returned.
func DiscoverWithLimit(projectPath string, limit int) ([]SessionMeta, error) {
	dir, err := ProjectSessionDir(projectPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no Claude Code sessions found for project %s (expected at %s)", projectPath, dir)
		}
		return nil, fmt.Errorf("cannot read session directory %s: %w", dir, err)
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			continue
		}

		meta, err := ExtractMeta(filePath)
		if err != nil {
			continue
		}
		meta.ProjectPath = projectPath
		sessions = append(sessions, *meta)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModifiedAt.After(sessions[j].ModifiedAt)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// FindSession finds a single session by ID or prefix within the project
// directory. Returns an error if no match or multiple matches are found.
func FindSession(projectPath, sessionID string) (*SessionMeta, error) {
	dir, err := ProjectSessionDir(projectPath)
	if err != nil {
		return nil, err
	}

	// Try exact match first.
	exact := filepath.Join(dir, sessionID+".jsonl")
	if _, err := os.Stat(exact); err == nil {
		meta, err := ExtractMeta(exact)
		if err != nil {
			return nil, fmt.Errorf("cannot parse session %s: %w", sessionID, err)
		}
		meta.ProjectPath = projectPath
		return meta, nil
	}

	// Prefix match.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read session directory: %w", err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		if strings.HasPrefix(name, sessionID) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session %q not found; use 'session list' to see available sessions", sessionID)
	case 1:
		filePath := filepath.Join(dir, matches[0]+".jsonl")
		meta, err := ExtractMeta(filePath)
		if err != nil {
			return nil, fmt.Errorf("cannot parse session %s: %w", matches[0], err)
		}
		meta.ProjectPath = projectPath
		return meta, nil
	default:
		return nil, fmt.Errorf("session ID %q is ambiguous; matches: %s", sessionID, strings.Join(matches, ", "))
	}
}

// ReadMessages reads and parses user and assistant messages from a session
// file. Supports pagination via offset and limit. If limit is 0, all
// messages are returned.
func ReadMessages(filePath string, offset, limit int, full bool) ([]Message, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	var messages []Message
	seen := make(map[string]bool)

	for _, line := range lines {
		obj, err := ParseLine(line)
		if err != nil || obj == nil {
			continue
		}

		typ, _ := obj["type"].(string)
		if typ != "user" && typ != "assistant" {
			continue
		}

		uuid, _ := obj["uuid"].(string)
		if uuid != "" && seen[uuid] {
			continue
		}
		if uuid != "" {
			seen[uuid] = true
		}

		msg := obj["message"]
		if msg == nil {
			continue
		}
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		content := ExtractContent(typ, msgMap["content"], full)
		if content == "" {
			continue
		}

		var toolUses []ToolUse
		if typ == "assistant" {
			toolUses = extractToolUses(msgMap["content"])
		}

		ts, _ := obj["timestamp"].(string)
		parsed, _ := time.Parse(time.RFC3339Nano, ts)

		messages = append(messages, Message{
			Type:      typ,
			Timestamp: parsed,
			Content:   content,
			UUID:      uuid,
			ToolUses:  toolUses,
		})
	}

	// Apply pagination.
	if offset > 0 {
		if offset >= len(messages) {
			return nil, nil
		}
		messages = messages[offset:]
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil
}

// Search scans sessions for the given query string (case-insensitive).
// mode: "title" (first message only), "user", "assistant", "all".
func Search(projectPath, query, mode string, limit int) ([]SearchResult, error) {
	sessions, err := Discover(projectPath)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for _, meta := range sessions {
		if limit > 0 && len(results) >= limit {
			break
		}

		matches := searchSession(meta.FilePath, queryLower, mode)
		if len(matches) > 0 {
			results = append(results, SearchResult{
				Meta:    meta,
				Matches: matches,
			})
		}
	}

	return results, nil
}

func searchSession(filePath, queryLower, mode string) []Match {
	lines, err := readLines(filePath)
	if err != nil {
		return nil
	}

	var matches []Match
	msgIndex := 0
	seen := make(map[string]bool)

	for _, line := range lines {
		obj, err := ParseLine(line)
		if err != nil || obj == nil {
			continue
		}

		typ, _ := obj["type"].(string)
		if typ != "user" && typ != "assistant" {
			continue
		}

		uuid, _ := obj["uuid"].(string)
		if uuid != "" && seen[uuid] {
			continue
		}
		if uuid != "" {
			seen[uuid] = true
		}

		msg := obj["message"]
		if msg == nil {
			continue
		}
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		content := ExtractContent(typ, msgMap["content"], false)
		if content == "" {
			continue
		}

		shouldSearch := false
		switch mode {
		case "title":
			shouldSearch = (typ == "user" && msgIndex == 0)
		case "user":
			shouldSearch = (typ == "user")
		case "assistant":
			shouldSearch = (typ == "assistant")
		default: // "all"
			shouldSearch = true
		}

		if shouldSearch {
			contentLower := strings.ToLower(content)
			if idx := strings.Index(contentLower, queryLower); idx >= 0 {
				preview := extractPreview(content, idx, len(queryLower))
				matches = append(matches, Match{
					Type:    typ,
					Preview: preview,
					Index:   msgIndex,
				})
			}
		}

		msgIndex++
	}

	return matches
}

func extractPreview(content string, matchIdx, matchLen int) string {
	const contextChars = 60

	start := matchIdx - contextChars
	if start < 0 {
		start = 0
	}
	end := matchIdx + matchLen + contextChars
	if end > len(content) {
		end = len(content)
	}

	preview := content[start:end]
	if start > 0 {
		preview = "..." + preview
	}
	if end < len(content) {
		preview = preview + "..."
	}

	return strings.ReplaceAll(preview, "\n", " ")
}

func readLines(filePath string) ([][]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read session file: %w", err)
	}

	var lines [][]byte
	for _, line := range splitLines(data) {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
