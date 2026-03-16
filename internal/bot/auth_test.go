package bot

import (
	"testing"
)

func TestAuthorizer_IsAllowed(t *testing.T) {
	auth := NewAuthorizer([]int64{123, 456, 789})

	tests := []struct {
		userID int64
		want   bool
	}{
		{123, true},
		{456, true},
		{789, true},
		{999, false},
		{0, false},
	}

	for _, tt := range tests {
		got := auth.IsAllowed(tt.userID)
		if got != tt.want {
			t.Errorf("IsAllowed(%d) = %v, want %v", tt.userID, got, tt.want)
		}
	}
}

func TestAuthorizer_EmptyList(t *testing.T) {
	auth := NewAuthorizer([]int64{})

	if auth.IsAllowed(123) {
		t.Error("IsAllowed() = true for empty list, want false")
	}
}
