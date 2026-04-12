# Session

The session package discovers, parses, and analyzes conversation sessions from Claude Code and Codex CLI. It enables browsing, searching, and exporting AI assistant session history across projects.

## Discovery

Sessions are discovered from two locations:
- **Claude Code**: `~/.claude/projects/<encoded-path>/` — JSONL files, one per session
- **Codex**: `~/.codex/sessions/` — JSON files with conversation arrays

Project paths are encoded by replacing `/` with `-` via [[internal/session/session.go#EncodePath]]. The discovery layer finds all session files for the current project directory.

## Parsing

[[internal/session/parser.go#ExtractMeta]] streams each session file via `bufio.Scanner` to extract metadata (message count, token usage, timestamps, duration) without loading all messages into memory. Individual lines are parsed by [[internal/session/parser.go#ParseLine]].

Codex sessions have two format generations handled by [[internal/session/codex.go]]:
- **Old format** (pre-Jan 2026): `response_item`, `session_meta`, function calls
- **New format**: Normalized structure with token counts

Both are transparently converted to the common [[internal/session/session.go#Message]] type.

## Commands

The session subcommand group provides browsing, search, export, and cleanup for session history.

- **list**: Show sessions with metadata (messages, tokens, duration)
- **show**: Display conversation history with optional tool-use expansion
- **search**: Full-text search across session content (title, user, assistant, or all)
- **export**: Render session as markdown document
- **resume**: Relaunch `claude` or `codex` with `--resume` for the given session
- **stats**: Aggregate statistics via [[internal/session/stats.go#Aggregate]] (total sessions, messages, tokens, duration, disk size)
- **clean**: Remove empty or old session files
