package repoconfig

import "fmt"

// WorkflowState represents a user-defined workflow state for a branch repo.
type WorkflowState string

const (
	StateActive        WorkflowState = "active"
	StateReview        WorkflowState = "review"
	StateApproved      WorkflowState = "approved"
	StateReviewBlocked WorkflowState = "review-blocked"
	StateMerged        WorkflowState = "merged"
	StateClosed        WorkflowState = "closed"
	StatePaused        WorkflowState = "paused"
)

// KnownStates returns the predefined workflow state names.
func KnownStates() []WorkflowState {
	return []WorkflowState{
		StateActive, StateReview, StateApproved, StateReviewBlocked,
		StateMerged, StateClosed, StatePaused,
	}
}

// IsKnownState returns true if s matches one of the predefined states.
func IsKnownState(s string) bool {
	for _, known := range KnownStates() {
		if string(known) == s {
			return true
		}
	}
	return false
}

// ValidateState checks that s is non-empty and contains only lowercase letters,
// digits, and hyphens.
func ValidateState(s string) error {
	if s == "" {
		return fmt.Errorf("state cannot be empty")
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return fmt.Errorf("state must contain only lowercase letters, digits, and hyphens; got %q", string(r))
		}
	}
	return nil
}
