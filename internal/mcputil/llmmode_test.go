package mcputil

import "testing"

func TestResolveLLMModeMatrix(t *testing.T) {
	tests := []struct {
		name        string
		client      *ClientInfo
		cfg         ResolverConfig
		interactive bool
		want        LLMMode
	}{
		{
			name:        "interactive agent → HostDelegated",
			client:      &ClientInfo{Name: "claude-code", Version: "1.0"},
			cfg:         ResolverConfig{},
			interactive: true,
			want:        ModeHostDelegated,
		},
		{
			name:        "sampling supported and enabled → Sampling",
			client:      &ClientInfo{Name: "test-client", SamplingSupported: true},
			cfg:         ResolverConfig{SamplingEnabled: true},
			interactive: false,
			want:        ModeSampling,
		},
		{
			name:        "sampling supported but NOT enabled → falls through",
			client:      &ClientInfo{Name: "test-client", SamplingSupported: true},
			cfg:         ResolverConfig{SamplingEnabled: false, ProviderConfigured: true},
			interactive: false,
			want:        ModeProvider,
		},
		{
			name:        "provider configured, no client → Provider",
			client:      nil,
			cfg:         ResolverConfig{ProviderConfigured: true},
			interactive: false,
			want:        ModeProvider,
		},
		{
			name:        "nothing available → Disabled",
			client:      nil,
			cfg:         ResolverConfig{},
			interactive: false,
			want:        ModeDisabled,
		},
		{
			name:        "interactive but no client → Disabled (defensive)",
			client:      nil,
			cfg:         ResolverConfig{},
			interactive: true,
			want:        ModeDisabled,
		},
		{
			name:        "codex client interactive → HostDelegated",
			client:      &ClientInfo{Name: "codex", Version: "0.1"},
			cfg:         ResolverConfig{ProviderConfigured: true},
			interactive: true,
			want:        ModeHostDelegated,
		},
		{
			name:        "non-interactive with provider → Provider (not host-delegated)",
			client:      &ClientInfo{Name: "cron-job"},
			cfg:         ResolverConfig{ProviderConfigured: true},
			interactive: false,
			want:        ModeProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveLLMMode(tt.client, tt.cfg, tt.interactive)
			if got != tt.want {
				t.Errorf("ResolveLLMMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLLMModeString(t *testing.T) {
	modes := map[LLMMode]string{
		ModeHostDelegated: "host-delegated",
		ModeSampling:      "sampling",
		ModeProvider:      "provider",
		ModeDisabled:      "disabled",
	}
	for mode, want := range modes {
		if got := mode.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", mode, got, want)
		}
	}
}
