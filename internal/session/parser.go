package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ParseLine parses a single JSONL line into a raw map.
// Returns nil, nil for lines that should be skipped (empty or malformed).
func ParseLine(line []byte) (map[string]interface{}, error) {
	if len(line) == 0 {
		return nil, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(line, &obj); err != nil {
		return nil, nil // skip malformed lines
	}
	return obj, nil
}

// ExtractMeta reads a session file and builds SessionMeta without loading
// all messages into memory. Streams through the file to extract first
// message, count messages, sum tokens, and compute duration.
func ExtractMeta(filePath string) (*SessionMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open session file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat session file: %w", err)
	}

	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")

	meta := &SessionMeta{
		SessionID:  sessionID,
		FilePath:   filePath,
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // up to 10MB lines

	var firstTimestamp, lastTimestamp time.Time
	firstMessageFound := false
	seen := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		obj, err := ParseLine(line)
		if err != nil || obj == nil {
			continue
		}

		typ, _ := obj["type"].(string)

		// Parse timestamp.
		if ts, ok := obj["timestamp"].(string); ok && ts != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				if firstTimestamp.IsZero() {
					firstTimestamp = parsed
				}
				lastTimestamp = parsed
			}
		}

		switch typ {
		case "user", "assistant":
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

			meta.MessageCount++

			if !firstMessageFound && typ == "user" {
				meta.FirstMessage = truncate(content, 200)
				firstMessageFound = true
			}

		case "result":
			// Sum token usage from result entries.
			meta.Tokens.InputTokens += jsonInt64(obj, "input_tokens")
			meta.Tokens.OutputTokens += jsonInt64(obj, "output_tokens")
			meta.Tokens.CacheReadTokens += jsonInt64(obj, "cache_read_input_tokens")
			meta.Tokens.CacheCreationTokens += jsonInt64(obj, "cache_creation_input_tokens")
		}

		// Also check usage in assistant messages.
		if typ == "assistant" {
			if msg, ok := obj["message"].(map[string]interface{}); ok {
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					meta.Tokens.InputTokens += jsonInt64FromMap(usage, "input_tokens")
					meta.Tokens.OutputTokens += jsonInt64FromMap(usage, "output_tokens")
					meta.Tokens.CacheReadTokens += jsonInt64FromMap(usage, "cache_read_input_tokens")
					meta.Tokens.CacheCreationTokens += jsonInt64FromMap(usage, "cache_creation_input_tokens")
				}
			}
		}
	}

	if !firstTimestamp.IsZero() {
		meta.CreatedAt = firstTimestamp
	}
	if !firstTimestamp.IsZero() && !lastTimestamp.IsZero() {
		meta.DurationSecs = lastTimestamp.Sub(firstTimestamp).Seconds()
	}

	return meta, scanner.Err()
}

// ExtractContent extracts displayable text content from a message.
// typ is "user" or "assistant". content is the raw message.content value.
// When full is false, tool_use blocks are summarized as [tool: <name>].
func ExtractContent(typ string, content interface{}, full bool) string {
	if content == nil {
		return ""
	}

	// String content (common for user messages).
	if s, ok := content.(string); ok {
		return s
	}

	// Array content (common for assistant messages).
	arr, ok := content.([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, block := range arr {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "text":
			if text, ok := blockMap["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		case "tool_use":
			name, _ := blockMap["name"].(string)
			if name == "" {
				name = "unknown"
			}
			if full {
				inputJSON := ""
				if input, ok := blockMap["input"]; ok {
					if b, err := json.Marshal(input); err == nil {
						inputJSON = string(b)
					}
				}
				parts = append(parts, fmt.Sprintf("[tool: %s] %s", name, inputJSON))
			} else {
				parts = append(parts, fmt.Sprintf("[tool: %s]", name))
			}
		case "tool_result":
			if full {
				if resultContent, ok := blockMap["content"].(string); ok {
					parts = append(parts, fmt.Sprintf("[result] %s", truncate(resultContent, 500)))
				}
			}
		case "thinking":
			// Skip thinking blocks in display.
		}
	}

	return strings.Join(parts, "\n")
}

func extractToolUses(content interface{}) []ToolUse {
	arr, ok := content.([]interface{})
	if !ok {
		return nil
	}

	var tools []ToolUse
	for _, block := range arr {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
			name, _ := blockMap["name"].(string)
			if name != "" {
				tools = append(tools, ToolUse{Name: name})
			}
		}
	}
	return tools
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func jsonInt64(obj map[string]interface{}, key string) int64 {
	return jsonInt64FromMap(obj, key)
}

func jsonInt64FromMap(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}
