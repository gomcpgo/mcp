package protocol

import (
	"encoding/json"
	"testing"
)

// TestProgressParams_MarshalRequiredOnly asserts that a minimal ProgressParams
// (no total, no message) serialises cleanly with only the mandatory fields.
func TestProgressParams_MarshalRequiredOnly(t *testing.T) {
	p := ProgressParams{
		ProgressToken: "abc",
		Progress:      42,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["progressToken"] != "abc" {
		t.Errorf("progressToken = %v", parsed["progressToken"])
	}
	if parsed["progress"].(float64) != 42 {
		t.Errorf("progress = %v", parsed["progress"])
	}
	if _, hasTotal := parsed["total"]; hasTotal {
		t.Error("total should be omitted when nil")
	}
	if _, hasMessage := parsed["message"]; hasMessage {
		t.Error("message should be omitted when empty")
	}
}

// TestProgressParams_MarshalWithTotalAndMessage covers a full progress update
// including an optional total and a human-readable message.
func TestProgressParams_MarshalWithTotalAndMessage(t *testing.T) {
	total := 100.0
	p := ProgressParams{
		ProgressToken: 42, // numeric tokens are legal per spec
		Progress:      25,
		Total:         &total,
		Message:       "quarter done",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["total"].(float64) != 100 {
		t.Errorf("total = %v, want 100", parsed["total"])
	}
	if parsed["message"] != "quarter done" {
		t.Errorf("message = %v", parsed["message"])
	}
	// Numeric tokens survive a round-trip.
	if parsed["progressToken"].(float64) != 42 {
		t.Errorf("progressToken = %v, want 42", parsed["progressToken"])
	}
}

// TestNotificationProgressConstant pins the spec-defined method name.
func TestNotificationProgressConstant(t *testing.T) {
	if NotificationProgress != "notifications/progress" {
		t.Errorf("NotificationProgress = %q, want notifications/progress", NotificationProgress)
	}
}
