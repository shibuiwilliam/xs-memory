package mcp

import (
	"path/filepath"
	"testing"

	"github.com/xs-memory/xs-memory/internal/mcputil"
	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/xsmem"
)

func openTestStore(t *testing.T, opts ...xsmem.Option) *xsmem.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xsmem")
	s, err := xsmem.Open(path, opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestResolveLLMModeInteractive(t *testing.T) {
	store := openTestStore(t)
	srv := NewServer(store, WithProviderConfigured(true))

	// Before any client connects → provider mode (non-interactive).
	mode := srv.ResolveLLMMode()
	if mode != mcputil.ModeProvider {
		t.Errorf("no client → mode=%v, want Provider", mode)
	}

	// After client connects → host-delegated (interactive).
	srv.SetClientInfo("claude-code", "1.0", false)
	mode = srv.ResolveLLMMode()
	if mode != mcputil.ModeHostDelegated {
		t.Errorf("with client → mode=%v, want HostDelegated", mode)
	}
}

func TestResolveLLMModeDisabled(t *testing.T) {
	store := openTestStore(t)
	srv := NewServer(store)
	// No client, no provider → disabled.
	mode := srv.ResolveLLMMode()
	if mode != mcputil.ModeDisabled {
		t.Errorf("nothing configured → mode=%v, want Disabled", mode)
	}
}

func TestServerCreation(t *testing.T) {
	// Verify all tools are registered without panicking.
	store := openTestStore(t)
	srv := NewServer(store)
	if srv.mcpServer == nil {
		t.Fatal("MCP server should be initialized")
	}
}

func TestHostDelegatedModeWithFailsafe(t *testing.T) {
	// Inject a failsafe provider. Verify mode resolves correctly.
	failsafe := provider.NewFailsafeLLM(func(msg string) {
		t.Fatal(msg)
	})
	store := openTestStore(t, xsmem.WithLLM(failsafe))
	srv := NewServer(store, WithProviderConfigured(true))
	srv.SetClientInfo("claude-code", "1.0", false)

	mode := srv.ResolveLLMMode()
	if mode != mcputil.ModeHostDelegated {
		t.Errorf("mode=%v, want HostDelegated", mode)
	}
}

func TestSamplingModeResolution(t *testing.T) {
	store := openTestStore(t)
	srv := NewServer(store)
	srv.resolverCfg.SamplingEnabled = true
	srv.SetClientInfo("test-client", "1.0", true) // supports sampling

	// Non-interactive + sampling client + opt-in → Sampling would be
	// expected, but since we set clientInfo (which makes it interactive),
	// it resolves to HostDelegated.
	mode := srv.ResolveLLMMode()
	if mode != mcputil.ModeHostDelegated {
		t.Errorf("mode=%v, want HostDelegated (interactive overrides)", mode)
	}
}
