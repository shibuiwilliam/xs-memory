// Package mcputil provides MCP capability detection and LLM mode resolution.
// See addendum §3 for the tiered LLM strategy.
package mcputil

// LLMMode determines how judgment-requiring operations are executed.
// See addendum §3.1.
type LLMMode int

const (
	// ModeHostDelegated — Tier 1: agent-driven. xs-memory returns work packets;
	// the host agent's model does reasoning and calls back via write tools.
	// Zero extra credentials/cost. See addendum §3.
	ModeHostDelegated LLMMode = iota

	// ModeSampling — Tier 2: MCP sampling capability advertised + opted in.
	// Optional accelerator, off by default, deprecated upstream. See addendum §3, H2.
	ModeSampling

	// ModeProvider — Tier 3: server-side provider configured (Ollama default).
	// Used in CLI/daemon/headless mode. See addendum §3, H3.
	ModeProvider

	// ModeDisabled — no judgment available. Search still works; organization is
	// deferred and queued, never fails. See addendum §3.1, H4, H7.
	ModeDisabled
)

// String returns a human-readable name for the mode.
func (m LLMMode) String() string {
	switch m {
	case ModeHostDelegated:
		return "host-delegated"
	case ModeSampling:
		return "sampling"
	case ModeProvider:
		return "provider"
	case ModeDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ClientInfo records identity and capabilities from the MCP initialize handshake.
// See addendum §3.1.
type ClientInfo struct {
	Name    string
	Version string

	// SamplingSupported is true if the client advertised the sampling capability.
	SamplingSupported bool
}

// ResolverConfig controls LLM mode resolution. See addendum §6.
type ResolverConfig struct {
	// SamplingEnabled gates Tier 2. Off by default per H2.
	SamplingEnabled bool

	// ProviderConfigured is true when a server-side LLM provider is set up.
	ProviderConfigured bool
}

// ResolveLLMMode determines the execution mode for judgment-requiring operations.
// Called per-request context. See addendum §3.1.
//
// Resolution rule:
//  1. If invoked through an interactive agent (MCP tools / Skill) → HostDelegated
//  2. If client advertised sampling AND opt-in → Sampling
//  3. If a provider is configured → Provider
//  4. Else → Disabled (search works; organization queued)
func ResolveLLMMode(client *ClientInfo, cfg ResolverConfig, interactive bool) LLMMode {
	// Tier 1: interactive agent driving via MCP tools or Skill.
	if interactive && client != nil {
		return ModeHostDelegated
	}

	// Tier 2: sampling — only if client supports it AND operator opted in.
	if client != nil && client.SamplingSupported && cfg.SamplingEnabled {
		return ModeSampling
	}

	// Tier 3: server-side provider configured.
	if cfg.ProviderConfigured {
		return ModeProvider
	}

	// Tier 4: no judgment available.
	return ModeDisabled
}
