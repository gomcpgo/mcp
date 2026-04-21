package handler

import (
	"context"
	"testing"
)

// TestProgressReporterFromContext_DefaultIsNoop verifies that a handler
// calling Report on a freshly-made context gets a no-op reporter (never nil)
// so it can emit progress unconditionally.
func TestProgressReporterFromContext_DefaultIsNoop(t *testing.T) {
	r := ProgressReporterFromContext(context.Background())
	if r == nil {
		t.Fatal("ProgressReporterFromContext must not return nil")
	}
	// Calling Report on the default must not panic and must return nil.
	if err := r.Report(1, nil, ""); err != nil {
		t.Errorf("noop Report returned err = %v, want nil", err)
	}
}

// captureReporter records each Report call for assertion.
type captureReporter struct {
	calls []captured
}

type captured struct {
	progress float64
	total    *float64
	message  string
}

func (c *captureReporter) Report(progress float64, total *float64, message string) error {
	c.calls = append(c.calls, captured{progress: progress, total: total, message: message})
	return nil
}

// TestProgressReporter_WithAndFromContextRoundTrips confirms injection works.
func TestProgressReporter_WithAndFromContextRoundTrips(t *testing.T) {
	cap := &captureReporter{}
	ctx := WithProgressReporter(context.Background(), cap)

	r := ProgressReporterFromContext(ctx)
	if r == nil {
		t.Fatal("expected reporter to be present")
	}

	total := 100.0
	if err := r.Report(50, &total, "halfway"); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if len(cap.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(cap.calls))
	}
	got := cap.calls[0]
	if got.progress != 50 || got.message != "halfway" {
		t.Errorf("captured call = %+v", got)
	}
	if got.total == nil || *got.total != 100 {
		t.Errorf("captured total = %v, want 100", got.total)
	}
}
