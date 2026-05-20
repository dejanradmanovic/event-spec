package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- NewTransport ---

func TestNewTransport_Defaults(t *testing.T) {
	tr, err := NewTransport(ProviderConfig{})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	if tr.client.Timeout != defaultTimeout {
		t.Errorf("Timeout = %v, want %v", tr.client.Timeout, defaultTimeout)
	}
	if len(tr.retryable) != len(defaultRetryableCodes) {
		t.Errorf("retryable count = %d, want %d", len(tr.retryable), len(defaultRetryableCodes))
	}
}

func TestNewTransport_CustomRetryableCodes(t *testing.T) {
	tr, err := NewTransport(ProviderConfig{
		RetryConfig: RetryConfig{RetryableErrors: []int{429, 503}},
	})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	if !tr.retryable[429] || !tr.retryable[503] {
		t.Error("expected 429 and 503 to be retryable")
	}
	if tr.retryable[500] {
		t.Error("500 should not be retryable with custom codes [429, 503]")
	}
}

func TestNewTransport_InvalidProxyURL(t *testing.T) {
	_, err := NewTransport(ProviderConfig{
		ProxyMode: ProxyReverseProxy,
		ProxyURL:  "://bad url",
	})
	if err == nil {
		t.Error("expected error for malformed proxy URL")
	}
}

// --- Retry behaviour ---

func TestTransport_RetryOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		},
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (2 failures then success)", got)
	}
}

func TestTransport_RetryOn500(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
		},
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestTransport_MaxRetriesEnforced(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	const maxRetries = 2
	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     maxRetries,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
			Multiplier:     2.0,
		},
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if resp != nil {
		resp.Body.Close()
		t.Error("expected nil response after exhausted retries")
	}
	if err == nil {
		t.Error("expected non-nil error after exhausted retries")
	}
	if got := atomic.LoadInt32(&calls); got != maxRetries+1 {
		t.Errorf("calls = %d, want %d (initial + %d retries)", got, maxRetries+1, maxRetries)
	}
}

func TestTransport_NoRetryOnNonRetryableCode(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest) // 400: not retryable
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{MaxRetries: 3, InitialBackoff: 1 * time.Millisecond},
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 400)", got)
	}
}

func TestTransport_NoRetryOn200(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{MaxRetries: 3, InitialBackoff: 1 * time.Millisecond},
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

// TestTransport_BackoffSequence verifies that delays grow exponentially between retries.
// Uses real wall-clock timing with a generous 50% floor tolerance for CI variance.
func TestTransport_BackoffSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	var mu sync.Mutex
	var timestamps []time.Time

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	const (
		initial    = 10 * time.Millisecond
		mult       = 2.0
		maxRetries = 3
	)
	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     maxRetries,
			InitialBackoff: initial,
			MaxBackoff:     1 * time.Second,
			Multiplier:     mult,
			Jitter:         false,
		},
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	tr.Do(context.Background(), req, nil) //nolint:errcheck

	mu.Lock()
	ts := make([]time.Time, len(timestamps))
	copy(ts, timestamps)
	mu.Unlock()

	if len(ts) != maxRetries+1 {
		t.Fatalf("calls = %d, want %d", len(ts), maxRetries+1)
	}

	// Verify each inter-request gap is at least half the nominal backoff.
	expected := initial
	for i := 1; i < len(ts); i++ {
		gap := ts[i].Sub(ts[i-1])
		floor := expected / 2
		if gap < floor {
			t.Errorf("gap[%d] = %v, want >= %v (half of %v nominal)", i, gap, floor, expected)
		}
		expected = time.Duration(float64(expected) * mult)
	}
}

// TestTransport_BodyReusedOnRetry verifies that POST body bytes are identical on every attempt.
func TestTransport_BodyReusedOnRetry(t *testing.T) {
	const wantBody = `{"event":"test"}`
	var calls int32
	var mu sync.Mutex
	var bodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
			Multiplier:     2.0,
		},
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := tr.Do(context.Background(), req, []byte(wantBody))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("no requests received")
	}
	for i, got := range bodies {
		if got != wantBody {
			t.Errorf("attempt %d: body = %q, want %q", i+1, got, wantBody)
		}
	}
}

// TestTransport_ContextCancellation verifies that an in-flight context cancellation
// is propagated to the caller and does not block.
func TestTransport_ContextCancellation(t *testing.T) {
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-srvCtx.Done():
		case <-time.After(10 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		srvCancel()
		srv.Close()
	}()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{MaxRetries: 3, InitialBackoff: 1 * time.Millisecond},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	_, err := tr.Do(ctx, req, nil)
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

// TestTransport_ContextCancelledBetweenRetries verifies that context expiry between
// retry attempts short-circuits the loop without making another HTTP request.
func TestTransport_ContextCancelledBetweenRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tr := mustTransport(t, ProviderConfig{
		RetryConfig: RetryConfig{
			MaxRetries:     5,
			InitialBackoff: 100 * time.Millisecond, // long enough for context to expire
			MaxBackoff:     1 * time.Second,
			Multiplier:     2.0,
		},
	})

	// Context expires after first attempt's response but before retry sleep completes.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	_, err := tr.Do(ctx, req, nil)
	if err == nil {
		t.Error("expected context error")
	}
	// Only the initial attempt should have been made before the context expired
	// during the retry sleep.
	if got := atomic.LoadInt32(&calls); got > 2 {
		t.Errorf("calls = %d, want <= 2 (context should have cancelled retry sleep)", got)
	}
}

// --- Proxy routing ---

func TestTransport_RewriteURL_Direct(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{ProxyMode: ProxyDirect})
	const providerURL = "https://api.amplitude.com/2/httpapi"
	got, err := tr.RewriteURL(providerURL)
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	if got != providerURL {
		t.Errorf("got %q, want unchanged %q", got, providerURL)
	}
}

func TestTransport_RewriteURL_EmptyProxyMode(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{})
	const providerURL = "https://api.amplitude.com/2/httpapi"
	got, err := tr.RewriteURL(providerURL)
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	if got != providerURL {
		t.Errorf("got %q, want unchanged %q", got, providerURL)
	}
}

func TestTransport_RewriteURL_ReverseProxy(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{
		ProxyMode: ProxyReverseProxy,
		ProxyURL:  "https://analytics.example.com/amp",
	})
	got, err := tr.RewriteURL("https://api2.amplitude.com/2/httpapi")
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	want := "https://analytics.example.com/amp/2/httpapi"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTransport_RewriteURL_ReverseProxy_TrailingSlash(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{
		ProxyMode: ProxyReverseProxy,
		ProxyURL:  "https://analytics.example.com/amp/",
	})
	got, err := tr.RewriteURL("https://api2.amplitude.com/2/httpapi")
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	want := "https://analytics.example.com/amp/2/httpapi"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTransport_RewriteURL_ReverseProxy_NoProxyPath(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{
		ProxyMode: ProxyReverseProxy,
		ProxyURL:  "https://analytics.example.com",
	})
	got, err := tr.RewriteURL("https://api2.amplitude.com/2/httpapi")
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	want := "https://analytics.example.com/2/httpapi"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTransport_RewriteURL_Custom(t *testing.T) {
	tr := mustTransport(t, ProviderConfig{
		ProxyMode: ProxyCustom,
		ProxyURL:  "https://custom.example.com/track",
	})
	got, err := tr.RewriteURL("https://api.amplitude.com/2/httpapi")
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	if got != "https://custom.example.com/track" {
		t.Errorf("got %q, want custom URL", got)
	}
}

func TestTransport_ProxyRouting_ReverseProxy(t *testing.T) {
	proxyCalled := false
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	tr := mustTransport(t, ProviderConfig{
		ProxyMode: ProxyReverseProxy,
		ProxyURL:  proxyServer.URL + "/proxy",
	})

	// Simulate what a provider does: rewrite the provider URL, then call Do.
	rewritten, err := tr.RewriteURL("https://api.amplitude.com/2/httpapi")
	if err != nil {
		t.Fatalf("RewriteURL: %v", err)
	}
	if !strings.HasPrefix(rewritten, proxyServer.URL) {
		t.Errorf("rewritten URL %q does not point to proxy server %q", rewritten, proxyServer.URL)
	}

	req, _ := http.NewRequest(http.MethodPost, rewritten, nil)
	resp, err := tr.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if !proxyCalled {
		t.Error("proxy server was not called")
	}
}

// --- Secret resolution ---

func TestResolveSecret_Inline(t *testing.T) {
	got, err := ResolveSecret("my-api-key", SecretInline)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "my-api-key" {
		t.Errorf("got %q, want %q", got, "my-api-key")
	}
}

func TestResolveSecret_EmptyType(t *testing.T) {
	got, err := ResolveSecret("my-api-key", "")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "my-api-key" {
		t.Errorf("got %q, want %q", got, "my-api-key")
	}
}

func TestResolveSecret_EnvVar_BracketSyntax(t *testing.T) {
	const varName = "TEST_RESOLVE_SECRET_BRACKET"
	t.Setenv(varName, "secret-value")

	got, err := ResolveSecret("${"+varName+"}", SecretEnvVar)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "secret-value" {
		t.Errorf("got %q, want %q", got, "secret-value")
	}
}

func TestResolveSecret_EnvVar_PlainName(t *testing.T) {
	const varName = "TEST_RESOLVE_SECRET_PLAIN"
	t.Setenv(varName, "plain-secret")

	got, err := ResolveSecret(varName, SecretEnvVar)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "plain-secret" {
		t.Errorf("got %q, want %q", got, "plain-secret")
	}
}

func TestResolveSecret_EnvVar_NotSet(t *testing.T) {
	os.Unsetenv("TEST_RESOLVE_SECRET_MISSING")
	_, err := ResolveSecret("${TEST_RESOLVE_SECRET_MISSING}", SecretEnvVar)
	if err == nil {
		t.Error("expected error for unset env var")
	}
}

func TestResolveSecret_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api_key.txt")
	if err := os.WriteFile(path, []byte("  file-secret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ResolveSecret(path, SecretFile)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "file-secret" {
		t.Errorf("got %q, want %q (trimmed)", got, "file-secret")
	}
}

func TestResolveSecret_File_NotFound(t *testing.T) {
	_, err := ResolveSecret("/nonexistent/path/key.txt", SecretFile)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResolveSecret_Vault_NotImplemented(t *testing.T) {
	_, err := ResolveSecret("vault:secret/data/key#value", SecretVault)
	if err == nil {
		t.Error("expected error: vault not implemented in Phase 1")
	}
}

func TestResolveSecret_UnknownType(t *testing.T) {
	_, err := ResolveSecret("key", SecretType("magic"))
	if err == nil {
		t.Error("expected error for unknown secret type")
	}
}

// --- helpers ---

func mustTransport(t *testing.T, cfg ProviderConfig) *Transport {
	t.Helper()
	tr, err := NewTransport(cfg)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	return tr
}
