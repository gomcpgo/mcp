package server

import (
	"strings"
	"testing"
)

// truncatedJSON must cap oversized payloads (e.g. base64 image data) so
// request/response logging never emits huge stderr lines that line-based
// log capture in the host cannot handle.
func TestTruncatedJSONCapsLargePayloads(t *testing.T) {
	big := map[string]string{"data": strings.Repeat("A", 100*1024)}
	out := truncatedJSON(big)

	if len(out) > maxLoggedJSONBytes+100 {
		t.Errorf("truncatedJSON output is %d bytes, want at most ~%d", len(out), maxLoggedJSONBytes)
	}
	if !strings.Contains(out, "truncated") {
		t.Error("truncated output does not carry a truncation marker")
	}
}

func TestTruncatedJSONLeavesSmallPayloadsUnchanged(t *testing.T) {
	small := map[string]string{"key": "value"}
	if got, want := truncatedJSON(small), PrettyJSON(small); got != want {
		t.Errorf("small payload was altered: got %q, want %q", got, want)
	}
}
