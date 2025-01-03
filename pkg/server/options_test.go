package server

import (
	"testing"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/transport"
)

func TestServerOptions(t *testing.T) {
	// Test default options
	defaults := DefaultOptions()
	if defaults.Name != "mcp-server" {
		t.Errorf("default Name = %v, want %v", defaults.Name, "mcp-server")
	}
	if defaults.Version != "1.0.0" {
		t.Errorf("default Version = %v, want %v", defaults.Version, "1.0.0")
	}
	if defaults.Registry == nil {
		t.Error("default Registry is nil")
	}
	if defaults.Transport == nil {
		t.Error("default Transport is nil")
	}

	// Test WithName option
	opt := Options{}
	WithName("test-server")(&opt)
	if opt.Name != "test-server" {
		t.Errorf("WithName() = %v, want %v", opt.Name, "test-server")
	}

	// Test WithVersion option
	WithVersion("2.0.0")(&opt)
	if opt.Version != "2.0.0" {
		t.Errorf("WithVersion() = %v, want %v", opt.Version, "2.0.0")
	}

	// Test WithRegistry option
	registry := handler.NewHandlerRegistry()
	WithRegistry(registry)(&opt)
	if opt.Registry != registry {
		t.Error("WithRegistry() did not set registry correctly")
	}

	// Test WithTransport option
	transport := transport.NewStdioTransport()
	WithTransport(transport)(&opt)
	if opt.Transport != transport {
		t.Error("WithTransport() did not set transport correctly")
	}

	// Test chaining options
	var opts Options
	for _, o := range []Option{
		WithName("chain-test"),
		WithVersion("3.0.0"),
		WithRegistry(registry),
		WithTransport(transport),
	} {
		o(&opts)
	}

	if opts.Name != "chain-test" {
		t.Errorf("chained Name = %v, want %v", opts.Name, "chain-test")
	}
	if opts.Version != "3.0.0" {
		t.Errorf("chained Version = %v, want %v", opts.Version, "3.0.0")
	}
	if opts.Registry != registry {
		t.Error("chained Registry not set correctly")
	}
	if opts.Transport != transport {
		t.Error("chained Transport not set correctly")
	}
}

func TestServerOptionsValidation(t *testing.T) {
	// Test server creation with nil options
	server := New(Options{})
	if server.options.Name == "" {
		t.Error("server should have default name when none provided")
	}
	if server.options.Version == "" {
		t.Error("server should have default version when none provided")
	}
	if server.registry == nil {
		t.Error("server should have default registry when none provided")
	}
	if server.transport == nil {
		t.Error("server should have default transport when none provided")
	}

	// Test server creation with partial options
	customRegistry := handler.NewHandlerRegistry()
	server = New(Options{
		Name:     "partial-test",
		Registry: customRegistry,
	})

	if server.options.Name != "partial-test" {
		t.Error("server should use provided name")
	}
	if server.options.Version == "" {
		t.Error("server should have default version when none provided")
	}
	if server.registry != customRegistry {
		t.Error("server should use provided registry")
	}
	if server.transport == nil {
		t.Error("server should have default transport when none provided")
	}

	// Test that options don't affect each other
	opt1 := Options{}
	opt2 := Options{}

	WithName("test1")(&opt1)
	WithName("test2")(&opt2)

	if opt1.Name == opt2.Name {
		t.Error("options should not affect each other")
	}
}
