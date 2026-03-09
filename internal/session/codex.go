package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// codexRecord is a normalized representation of a Codex JSONL line,
// abstracting over both the old (pre-Jan 2026) and new formats.
type codexRecord struct {
	Timestamp string // RFC3339Nano

	// Exactly one of these is set:
	SessionHeader *codexSessionHeader // first line (old: top-level id/timestamp; new: session_meta)
	Message       *codexMessage       // user or assistant message
	FunctionCall  *codexFunctionCall  // tool invocation
	FunctionOut   *codexFunctionOut   // tool result
	TokenCount    *codexTokenCount    // cumulative token usage from event_msg token_count
	// Other record types (reasoning, event_msg state, etc.) are dropped.
}

type codexSessionHeader struct {
	ID  string
	CWD string // empty for old format (cwd is in environment_context)
}

type codexMessage struct {
	Role string // "user", "assistant", "developer"
	Text string
}

type codexFunctionCall struct {
	Name      string
	Arguments string
	CallID    string
}

type codexFunctionOut struct {
	CallID string
	Output string
}

type codexTokenCount struct {
	InputTokens    int64
	OutputTokens   int64
	CachedTokens   int64
	ReasonTokens   int64
}

// parseCodexLine normalises a single JSONL line from either old or new Codex format.
func parseCodexLine(line []byte) *codexRecord {
	var obj map[string]interface{}
	if err := json.Unmarshal(line, &obj); err != nil {
		return nil
	}

	ts, _ := obj["timestamp"].(string)

	typ, _ := obj["type"].(string)

	// --- New format (response_item / session_meta envelope) ---
	if typ == "session_meta" {
		payload, _ := obj["payload"].(map[string]interface{})
		if payload == nil {
			return nil
		}
		id, _ := payload["id"].(string)
		cwd, _ := payload["cwd"].(string)
		return &codexRecord{Timestamp: ts, SessionHeader: &codexSessionHeader{ID: id, CWD: cwd}}
	}

	if typ == "response_item" {
		payload, _ := obj["payload"].(map[string]interface{})
		if payload == nil {
			return nil
		}
		ptype, _ := payload["type"].(string)
		switch ptype {
		case "message":
			role, _ := payload["role"].(string)
			text := extractCodexMessageText(payload)
			if role == "" || text == "" {
				return nil
			}
			return &codexRecord{Timestamp: ts, Message: &codexMessage{Role: role, Text: text}}
		case "function_call":
			name, _ := payload["name"].(string)
			args, _ := payload["arguments"].(string)
			callID, _ := payload["call_id"].(string)
			return &codexRecord{Timestamp: ts, FunctionCall: &codexFunctionCall{Name: name, Arguments: args, CallID: callID}}
		case "function_call_output":
			callID, _ := payload["call_id"].(string)
			output, _ := payload["output"].(string)
			return &codexRecord{Timestamp: ts, FunctionOut: &codexFunctionOut{CallID: callID, Output: output}}
		}
		return nil
	}

	if typ == "event_msg" {
		payload, _ := obj["payload"].(map[string]interface{})
		if payload != nil {
			if ptype, _ := payload["type"].(string); ptype == "token_count" {
				return parseCodexTokenCount(ts, payload)
			}
		}
		return nil
	}

	// --- Old format (top-level records, no envelope) ---

	// Header line: has "id" key at top level, no "type".
	if id, ok := obj["id"].(string); ok && typ == "" {
		return &codexRecord{Timestamp: ts, SessionHeader: &codexSessionHeader{ID: id}}
	}

	switch typ {
	case "message":
		role, _ := obj["role"].(string)
		text := extractCodexMessageText(obj)
		if role == "" || text == "" {
			return nil
		}
		return &codexRecord{Timestamp: ts, Message: &codexMessage{Role: role, Text: text}}
	case "function_call":
		name, _ := obj["name"].(string)
		args, _ := obj["arguments"].(string)
		callID, _ := obj["call_id"].(string)
		return &codexRecord{Timestamp: ts, FunctionCall: &codexFunctionCall{Name: name, Arguments: args, CallID: callID}}
	case "function_call_output":
		callID, _ := obj["call_id"].(string)
		output, _ := obj["output"].(string)
		return &codexRecord{Timestamp: ts, FunctionOut: &codexFunctionOut{CallID: callID, Output: output}}
	}

	return nil
}

var cwdRegexp = regexp.MustCompile(`<cwd>([^<]+)</cwd>`)

// CodexSessionsDir returns the path to ~/.codex/sessions/.
func CodexSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

// DiscoverCodex finds Codex sessions whose cwd matches projectPath.
// Walks ~/.codex/sessions/ recursively. Returns sessions sorted by
// ModifiedAt descending. If limit > 0, returns at most limit sessions.
func DiscoverCodex(projectPath string, limit int) ([]SessionMeta, error) {
	dir, err := CodexSessionsDir()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []SessionMeta

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info.Size() == 0 {
			return nil
		}

		meta, err := ExtractCodexMeta(path)
		if err != nil {
			return nil
		}

		// Filter by project path.
		if projectPath != "" && !PathContains(projectPath, meta.ProjectPath) {
			return nil
		}

		sessions = append(sessions, *meta)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot walk codex sessions: %w", err)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModifiedAt.After(sessions[j].ModifiedAt)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// FindCodexSession finds a single Codex session by ID or prefix.
func FindCodexSession(projectPath, sessionID string) (*SessionMeta, error) {
	dir, err := CodexSessionsDir()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("session %q not found; no codex sessions directory", sessionID)
	}

	var matches []SessionMeta

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info.Size() == 0 {
			return nil
		}

		// Extract UUID from filename: rollout-YYYY-MM-DDTHH-MM-SS-<uuid>.jsonl
		uuid := extractCodexUUID(filepath.Base(path))
		if uuid == "" {
			return nil
		}

		// Check exact or prefix match.
		if uuid != sessionID && !strings.HasPrefix(uuid, sessionID) {
			return nil
		}

		meta, err := ExtractCodexMeta(path)
		if err != nil {
			return nil
		}

		// Filter by project path if provided.
		if projectPath != "" && !PathContains(projectPath, meta.ProjectPath) {
			return nil
		}

		matches = append(matches, *meta)
		return nil
	})

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session %q not found in codex sessions", sessionID)
	case 1:
		return &matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.SessionID
		}
		return nil, fmt.Errorf("session ID %q is ambiguous; matches: %s", sessionID, strings.Join(ids, ", "))
	}
}

// ExtractCodexMeta reads a Codex session file and builds SessionMeta.
// Handles both old (pre-Jan 2026) and new Codex JSONL formats.
func ExtractCodexMeta(filePath string) (*SessionMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open codex session file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat codex session file: %w", err)
	}

	meta := &SessionMeta{
		CLI:        "codex",
		FilePath:   filePath,
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var firstTimestamp, lastTimestamp time.Time
	firstMessageFound := false

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		rec := parseCodexLine(line)
		if rec == nil {
			continue
		}

		if rec.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				if firstTimestamp.IsZero() {
					firstTimestamp = parsed
				}
				lastTimestamp = parsed
			}
		}

		if h := rec.SessionHeader; h != nil {
			meta.SessionID = h.ID
			if h.CWD != "" {
				meta.ProjectPath = h.CWD
			}
			continue
		}

		// Token counts are cumulative; keep overwriting so the last one wins.
		if tc := rec.TokenCount; tc != nil {
			meta.Tokens.InputTokens = tc.InputTokens
			meta.Tokens.OutputTokens = tc.OutputTokens
			meta.Tokens.CacheReadTokens = tc.CachedTokens
		}

		if m := rec.Message; m != nil {
			if m.Role == "developer" {
				continue
			}

			// For old format, extract cwd from environment_context if we don't have it yet.
			if m.Role == "user" && meta.ProjectPath == "" {
				if match := cwdRegexp.FindStringSubmatch(m.Text); match != nil {
					meta.ProjectPath = match[1]
				}
			}

			if isCodexSystemMessage(m.Text) {
				continue
			}

			if m.Role == "user" || m.Role == "assistant" {
				meta.MessageCount++
				if !firstMessageFound && m.Role == "user" {
					meta.FirstMessage = truncate(m.Text, 200)
					firstMessageFound = true
				}
			}
		}
	}

	// Fall back to extracting session ID from filename if header didn't provide one.
	if meta.SessionID == "" {
		meta.SessionID = extractCodexUUID(filepath.Base(filePath))
	}

	if !firstTimestamp.IsZero() {
		meta.CreatedAt = firstTimestamp
	}
	if !firstTimestamp.IsZero() && !lastTimestamp.IsZero() {
		meta.DurationSecs = lastTimestamp.Sub(firstTimestamp).Seconds()
	}

	return meta, scanner.Err()
}

// ReadCodexMessages reads and parses messages from a Codex session file.
// Handles both old (pre-Jan 2026) and new Codex JSONL formats.
func ReadCodexMessages(filePath string, offset, limit int, full bool) ([]Message, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read codex session file: %w", err)
	}

	var messages []Message
	var lastAssistant *Message // for attaching tool uses

	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}

		rec := parseCodexLine(line)
		if rec == nil {
			continue
		}

		parsed, _ := time.Parse(time.RFC3339Nano, rec.Timestamp)

		if m := rec.Message; m != nil {
			if m.Role == "developer" {
				continue
			}
			if m.Role != "user" && m.Role != "assistant" {
				continue
			}
			if isCodexSystemMessage(m.Text) {
				continue
			}

			msg := Message{
				Type:      m.Role,
				Timestamp: parsed,
				Content:   m.Text,
			}

			if m.Role == "assistant" {
				lastAssistant = &msg
			} else {
				lastAssistant = nil
			}
			messages = append(messages, msg)
			continue
		}

		if fc := rec.FunctionCall; fc != nil {
			name := fc.Name
			if name == "" {
				name = "unknown"
			}

			toolLine := fmt.Sprintf("[tool: %s]", name)
			if full && fc.Arguments != "" {
				toolLine = fmt.Sprintf("[tool: %s] %s", name, fc.Arguments)
			}

			toolUse := ToolUse{Name: name}

			if lastAssistant != nil {
				idx := len(messages) - 1
				messages[idx].Content += "\n" + toolLine
				messages[idx].ToolUses = append(messages[idx].ToolUses, toolUse)
			} else {
				msg := Message{
					Type:      "assistant",
					Timestamp: parsed,
					Content:   toolLine,
					ToolUses:  []ToolUse{toolUse},
				}
				lastAssistant = &msg
				messages = append(messages, msg)
			}
			continue
		}

		if fo := rec.FunctionOut; fo != nil && full {
			if fo.Output != "" {
				resultLine := fmt.Sprintf("[result] %s", truncate(fo.Output, 500))

				if lastAssistant != nil {
					idx := len(messages) - 1
					messages[idx].Content += "\n" + resultLine
				} else {
					messages = append(messages, Message{
						Type:      "assistant",
						Timestamp: parsed,
						Content:   resultLine,
					})
				}
			}
		}
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

// SearchCodex scans Codex sessions for the given query string.
func SearchCodex(projectPath, query, mode string, limit int) ([]SearchResult, error) {
	sessions, err := DiscoverCodex(projectPath, 0)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for _, meta := range sessions {
		if limit > 0 && len(results) >= limit {
			break
		}

		matches := searchCodexSession(meta.FilePath, queryLower, mode)
		if len(matches) > 0 {
			results = append(results, SearchResult{
				Meta:    meta,
				Matches: matches,
			})
		}
	}

	return results, nil
}

func searchCodexSession(filePath, queryLower, mode string) []Match {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var matches []Match
	msgIndex := 0

	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}

		rec := parseCodexLine(line)
		if rec == nil || rec.Message == nil {
			continue
		}

		m := rec.Message
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		if isCodexSystemMessage(m.Text) {
			continue
		}

		shouldSearch := false
		switch mode {
		case "title":
			shouldSearch = (m.Role == "user" && msgIndex == 0)
		case "user":
			shouldSearch = (m.Role == "user")
		case "assistant":
			shouldSearch = (m.Role == "assistant")
		default:
			shouldSearch = true
		}

		if shouldSearch {
			contentLower := strings.ToLower(m.Text)
			if idx := strings.Index(contentLower, queryLower); idx >= 0 {
				preview := extractPreview(m.Text, idx, len(queryLower))
				matches = append(matches, Match{
					Type:    m.Role,
					Preview: preview,
					Index:   msgIndex,
				})
			}
		}

		msgIndex++
	}

	return matches
}

// extractCodexMessageText extracts text from a Codex message payload.
// Content is an array of {type: "input_text"/"output_text", text: "..."}.
func extractCodexMessageText(payload map[string]interface{}) string {
	content, ok := payload["content"].([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, block := range content {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		blockType, _ := blockMap["type"].(string)
		if blockType == "input_text" || blockType == "output_text" {
			if text, ok := blockMap["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// parseCodexTokenCount extracts cumulative token usage from an event_msg
// token_count payload. The info.total_token_usage object is cumulative, so
// the last record in the file represents the session total.
func parseCodexTokenCount(ts string, payload map[string]interface{}) *codexRecord {
	info, _ := payload["info"].(map[string]interface{})
	if info == nil {
		return nil
	}
	usage, _ := info["total_token_usage"].(map[string]interface{})
	if usage == nil {
		return nil
	}
	return &codexRecord{
		Timestamp: ts,
		TokenCount: &codexTokenCount{
			InputTokens:  jsonInt64FromMap(usage, "input_tokens"),
			OutputTokens: jsonInt64FromMap(usage, "output_tokens"),
			CachedTokens: jsonInt64FromMap(usage, "cached_input_tokens"),
			ReasonTokens: jsonInt64FromMap(usage, "reasoning_output_tokens"),
		},
	}
}

// isCodexSystemMessage returns true if the text looks like a system-injected
// user message (AGENTS.md, environment_context, permissions, etc.) rather
// than an actual user prompt.
// PathContains reports whether child is equal to parent or is a subdirectory
// of parent.  It uses cleaned, absolute-style paths so that /work/foo does NOT
// match /work/foobar.
func PathContains(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	// Ensure the prefix ends with a separator so /work/foo won't match /work/foobar.
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(child, parent)
}

func isCodexSystemMessage(text string) bool {
	prefixes := []string{
		"# AGENTS.md",
		"<environment_context>",
		"<permissions instructions>",
		"<INSTRUCTIONS>",
		"<collaboration_mode>",
		"<user_instructions>",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

// extractCodexUUID extracts the UUID from a Codex session filename.
// Format: rollout-YYYY-MM-DDTHH-MM-SS-<uuid>.jsonl
// The UUID is the last 36 characters before .jsonl.
func extractCodexUUID(filename string) string {
	name := strings.TrimSuffix(filename, ".jsonl")
	// UUID is 36 chars: 8-4-4-4-12
	if len(name) < 36 {
		return ""
	}
	uuid := name[len(name)-36:]
	// Basic validation: check for hyphens at expected positions.
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		return ""
	}
	return uuid
}
