package session

import (
	"testing"
	"time"
)

func TestAggregate_Empty(t *testing.T) {
	stats := Aggregate(nil)
	if stats.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", stats.TotalSessions)
	}
	if stats.TotalMessages != 0 {
		t.Errorf("TotalMessages = %d, want 0", stats.TotalMessages)
	}
	if stats.TotalSizeBytes != 0 {
		t.Errorf("TotalSizeBytes = %d, want 0", stats.TotalSizeBytes)
	}
}

func TestAggregate_Single(t *testing.T) {
	created := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{
		{
			SessionID:    "abc123",
			MessageCount: 10,
			DurationSecs: 300,
			SizeBytes:    1024,
			CreatedAt:    created,
			Tokens: TokenUsage{
				InputTokens:         100,
				OutputTokens:        200,
				CacheReadTokens:     50,
				CacheCreationTokens: 25,
			},
		},
	}

	stats := Aggregate(sessions)

	if stats.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", stats.TotalSessions)
	}
	if stats.TotalMessages != 10 {
		t.Errorf("TotalMessages = %d, want 10", stats.TotalMessages)
	}
	if stats.TotalDurationSecs != 300 {
		t.Errorf("TotalDurationSecs = %v, want 300", stats.TotalDurationSecs)
	}
	if stats.TotalSizeBytes != 1024 {
		t.Errorf("TotalSizeBytes = %d, want 1024", stats.TotalSizeBytes)
	}
	if stats.TotalTokens.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", stats.TotalTokens.InputTokens)
	}
	if stats.TotalTokens.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", stats.TotalTokens.OutputTokens)
	}
	if stats.TotalTokens.CacheReadTokens != 50 {
		t.Errorf("CacheReadTokens = %d, want 50", stats.TotalTokens.CacheReadTokens)
	}
	if stats.TotalTokens.CacheCreationTokens != 25 {
		t.Errorf("CacheCreationTokens = %d, want 25", stats.TotalTokens.CacheCreationTokens)
	}
	if !stats.OldestSession.Equal(created) {
		t.Errorf("OldestSession = %v, want %v", stats.OldestSession, created)
	}
	if !stats.NewestSession.Equal(created) {
		t.Errorf("NewestSession = %v, want %v", stats.NewestSession, created)
	}
}

func TestAggregate_Multiple(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	sessions := []SessionMeta{
		{
			MessageCount: 5,
			DurationSecs: 100,
			SizeBytes:    500,
			CreatedAt:    t2,
			Tokens:       TokenUsage{InputTokens: 10, OutputTokens: 20},
		},
		{
			MessageCount: 15,
			DurationSecs: 200,
			SizeBytes:    1500,
			CreatedAt:    t1,
			Tokens:       TokenUsage{InputTokens: 30, OutputTokens: 40},
		},
		{
			MessageCount: 10,
			DurationSecs: 150,
			SizeBytes:    1000,
			CreatedAt:    t3,
			Tokens:       TokenUsage{InputTokens: 50, OutputTokens: 60},
		},
	}

	stats := Aggregate(sessions)

	if stats.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", stats.TotalSessions)
	}
	if stats.TotalMessages != 30 {
		t.Errorf("TotalMessages = %d, want 30", stats.TotalMessages)
	}
	if stats.TotalDurationSecs != 450 {
		t.Errorf("TotalDurationSecs = %v, want 450", stats.TotalDurationSecs)
	}
	if stats.TotalSizeBytes != 3000 {
		t.Errorf("TotalSizeBytes = %d, want 3000", stats.TotalSizeBytes)
	}
	if stats.TotalTokens.InputTokens != 90 {
		t.Errorf("InputTokens = %d, want 90", stats.TotalTokens.InputTokens)
	}
	if stats.TotalTokens.OutputTokens != 120 {
		t.Errorf("OutputTokens = %d, want 120", stats.TotalTokens.OutputTokens)
	}
	if !stats.OldestSession.Equal(t1) {
		t.Errorf("OldestSession = %v, want %v", stats.OldestSession, t1)
	}
	if !stats.NewestSession.Equal(t3) {
		t.Errorf("NewestSession = %v, want %v", stats.NewestSession, t3)
	}
}

func TestAggregate_OldestNewestOrder(t *testing.T) {
	early := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	// First session is the latest, second is the earliest.
	sessions := []SessionMeta{
		{CreatedAt: late},
		{CreatedAt: early},
	}

	stats := Aggregate(sessions)

	if !stats.OldestSession.Equal(early) {
		t.Errorf("OldestSession = %v, want %v", stats.OldestSession, early)
	}
	if !stats.NewestSession.Equal(late) {
		t.Errorf("NewestSession = %v, want %v", stats.NewestSession, late)
	}
}
