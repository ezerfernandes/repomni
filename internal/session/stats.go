package session

import "time"

// Stats holds aggregate statistics across multiple sessions.
type Stats struct {
	TotalSessions     int        `json:"total_sessions"`
	TotalMessages     int        `json:"total_messages"`
	TotalTokens       TokenUsage `json:"total_tokens"`
	TotalDurationSecs float64    `json:"total_duration_seconds"`
	TotalSizeBytes    int64      `json:"total_size_bytes"`
	OldestSession     time.Time  `json:"oldest_session"`
	NewestSession     time.Time  `json:"newest_session"`
}

// Aggregate computes Stats from a slice of SessionMeta.
func Aggregate(sessions []SessionMeta) Stats {
	if len(sessions) == 0 {
		return Stats{}
	}

	s := Stats{
		TotalSessions: len(sessions),
		OldestSession: sessions[0].CreatedAt,
		NewestSession: sessions[0].CreatedAt,
	}

	for _, meta := range sessions {
		s.TotalMessages += meta.MessageCount
		s.TotalDurationSecs += meta.DurationSecs
		s.TotalSizeBytes += meta.SizeBytes

		s.TotalTokens.InputTokens += meta.Tokens.InputTokens
		s.TotalTokens.OutputTokens += meta.Tokens.OutputTokens
		s.TotalTokens.CacheReadTokens += meta.Tokens.CacheReadTokens
		s.TotalTokens.CacheCreationTokens += meta.Tokens.CacheCreationTokens

		if !meta.CreatedAt.IsZero() && meta.CreatedAt.Before(s.OldestSession) {
			s.OldestSession = meta.CreatedAt
		}
		if meta.CreatedAt.After(s.NewestSession) {
			s.NewestSession = meta.CreatedAt
		}
	}

	return s
}
