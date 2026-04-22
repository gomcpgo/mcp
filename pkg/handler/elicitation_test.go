package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

type fakeElicitor struct {
	result *protocol.ElicitationResult
	err    error
	called bool
	gotMsg string
}

func (f *fakeElicitor) Elicit(_ context.Context, message string, _ json.RawMessage) (*protocol.ElicitationResult, error) {
	f.called = true
	f.gotMsg = message
	return f.result, f.err
}

func TestElicitorFromContext_NoElicitorReturnsStub(t *testing.T) {
	e := ElicitorFromContext(context.Background())
	if e == nil {
		t.Fatal("ElicitorFromContext returned nil; expected stub")
	}
	_, err := e.Elicit(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("stub Elicit should return an error when client lacks capability")
	}
	if !errors.Is(err, ErrElicitationNotSupported) {
		t.Errorf("err = %v, want ErrElicitationNotSupported", err)
	}
}

func TestElicitorFromContext_RealElicitorReached(t *testing.T) {
	fake := &fakeElicitor{
		result: &protocol.ElicitationResult{Action: protocol.ElicitationActionAccept},
	}
	ctx := WithElicitor(context.Background(), fake)

	e := ElicitorFromContext(ctx)
	result, err := e.Elicit(ctx, "hello", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fake.called {
		t.Error("fake elicitor was not called")
	}
	if fake.gotMsg != "hello" {
		t.Errorf("gotMsg = %q, want 'hello'", fake.gotMsg)
	}
	if result.Action != protocol.ElicitationActionAccept {
		t.Errorf("action = %q, want accept", result.Action)
	}
}

func TestWithElicitor_NilReturnsCtxUnchanged(t *testing.T) {
	parent := context.Background()
	got := WithElicitor(parent, nil)
	if got != parent {
		t.Error("WithElicitor(ctx, nil) should return the parent ctx unchanged")
	}
}
