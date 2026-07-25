package ai

import (
	"testing"
)

func TestGetCachedQuestions_NilDatabase(t *testing.T) {
	noteContent := "# Test\nContent"
	questions, suggestions, isCached, noteChanged, err := GetCachedQuestions(nil, 1, noteContent)

	if err != nil || questions != "" || isCached {
		t.Errorf("GetCachedQuestions should handle nil database gracefully")
	}
}

func TestGetCachedQuestions_InvalidCardID(t *testing.T) {
	noteContent := "# Test\nContent"
	questions, suggestions, isCached, noteChanged, err := GetCachedQuestions(nil, 0, noteContent)

	if err != nil || questions != "" || isCached {
		t.Errorf("GetCachedQuestions should handle invalid card ID gracefully")
	}
}
