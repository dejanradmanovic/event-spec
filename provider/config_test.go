package provider

import (
	"testing"
	"time"
)

func TestSecretTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value SecretType
		want  string
	}{
		{"env_var", SecretEnvVar, "env_var"},
		{"file", SecretFile, "file"},
		{"vault", SecretVault, "vault"},
		{"inline", SecretInline, "inline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("SecretType = %q, want %q", tt.value, tt.want)
			}
		})
	}
}

func TestProxyModeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value ProxyMode
		want  string
	}{
		{"direct", ProxyDirect, "direct"},
		{"reverse_proxy", ProxyReverseProxy, "reverse_proxy"},
		{"custom", ProxyCustom, "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("ProxyMode = %q, want %q", tt.value, tt.want)
			}
		})
	}
}

func TestOverflowPolicyConstants(t *testing.T) {
	tests := []struct {
		name  string
		value OverflowPolicy
		want  string
	}{
		{"drop_oldest", OverflowDropOldest, "drop_oldest"},
		{"drop_newest", OverflowDropNewest, "drop_newest"},
		{"block", OverflowBlock, "block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("OverflowPolicy = %q, want %q", tt.value, tt.want)
			}
		})
	}
}

func TestRetryConfig_zeroValue(t *testing.T) {
	var rc RetryConfig
	if rc.MaxRetries != 0 {
		t.Errorf("MaxRetries zero value = %d, want 0", rc.MaxRetries)
	}
	if rc.InitialBackoff != 0 {
		t.Errorf("InitialBackoff zero value = %v, want 0", rc.InitialBackoff)
	}
	if rc.Jitter {
		t.Error("Jitter zero value = true, want false")
	}
	if len(rc.RetryableErrors) != 0 {
		t.Errorf("RetryableErrors zero value len = %d, want 0", len(rc.RetryableErrors))
	}
}

func TestRetryConfig_fields(t *testing.T) {
	rc := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      30 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
		RetryableErrors: []int{429, 500, 502, 503, 504},
	}
	if rc.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rc.MaxRetries)
	}
	if rc.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", rc.InitialBackoff)
	}
	if rc.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want 30s", rc.MaxBackoff)
	}
	if rc.Multiplier != 2.0 {
		t.Errorf("Multiplier = %g, want 2.0", rc.Multiplier)
	}
	if !rc.Jitter {
		t.Error("Jitter = false, want true")
	}
	if len(rc.RetryableErrors) != 5 {
		t.Errorf("len(RetryableErrors) = %d, want 5", len(rc.RetryableErrors))
	}
}

func TestRateLimitConfig_fields(t *testing.T) {
	rl := RateLimitConfig{
		RequestsPerSecond: 30,
		BurstSize:         60,
	}
	if rl.RequestsPerSecond != 30 {
		t.Errorf("RequestsPerSecond = %d, want 30", rl.RequestsPerSecond)
	}
	if rl.BurstSize != 60 {
		t.Errorf("BurstSize = %d, want 60", rl.BurstSize)
	}
}

func TestProviderConfig_fields(t *testing.T) {
	cfg := ProviderConfig{
		APIKey:         "${AMPLITUDE_API_KEY}",
		SecretType:     SecretEnvVar,
		ProxyURL:       "https://analytics.example.com/amp",
		ProxyMode:      ProxyReverseProxy,
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		MaxQueueSize:   10000,
		OverflowPolicy: OverflowDropOldest,
		RetryConfig: RetryConfig{
			MaxRetries: 3,
			Jitter:     true,
		},
		RateLimitConfig: RateLimitConfig{
			RequestsPerSecond: 30,
			BurstSize:         60,
		},
		Timeout:      5 * time.Second,
		MaxIdleConns: 10,
	}

	if cfg.APIKey != "${AMPLITUDE_API_KEY}" {
		t.Errorf("APIKey = %q, want ${AMPLITUDE_API_KEY}", cfg.APIKey)
	}
	if cfg.SecretType != SecretEnvVar {
		t.Errorf("SecretType = %q, want %q", cfg.SecretType, SecretEnvVar)
	}
	if cfg.ProxyURL != "https://analytics.example.com/amp" {
		t.Errorf("ProxyURL = %q, want https://analytics.example.com/amp", cfg.ProxyURL)
	}
	if cfg.ProxyMode != ProxyReverseProxy {
		t.Errorf("ProxyMode = %q, want %q", cfg.ProxyMode, ProxyReverseProxy)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", cfg.BatchSize)
	}
	if cfg.FlushInterval != 10*time.Second {
		t.Errorf("FlushInterval = %v, want 10s", cfg.FlushInterval)
	}
	if cfg.MaxQueueSize != 10000 {
		t.Errorf("MaxQueueSize = %d, want 10000", cfg.MaxQueueSize)
	}
	if cfg.OverflowPolicy != OverflowDropOldest {
		t.Errorf("OverflowPolicy = %q, want %q", cfg.OverflowPolicy, OverflowDropOldest)
	}
	if cfg.RetryConfig.MaxRetries != 3 {
		t.Errorf("RetryConfig.MaxRetries = %d, want 3", cfg.RetryConfig.MaxRetries)
	}
	if cfg.RateLimitConfig.RequestsPerSecond != 30 {
		t.Errorf("RateLimitConfig.RequestsPerSecond = %d, want 30", cfg.RateLimitConfig.RequestsPerSecond)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.MaxIdleConns)
	}
	if cfg.TLSConfig != nil {
		t.Error("TLSConfig zero value should be nil")
	}
}
