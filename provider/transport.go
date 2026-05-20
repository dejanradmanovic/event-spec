package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultMaxRetries     = 3
	defaultInitialBackoff = 100 * time.Millisecond
	defaultMaxBackoff     = 30 * time.Second
	defaultMultiplier     = 2.0
	defaultTimeout        = 30 * time.Second
	defaultMaxIdleConns   = 100
)

var defaultRetryableCodes = []int{429, 500, 502, 503, 504}

// Transport is a shared HTTP client with retry, exponential backoff, and proxy support
// used by all provider implementations.
type Transport struct {
	client    *http.Client
	cfg       ProviderConfig
	retryable map[int]bool
}

// NewTransport constructs a Transport from cfg.
// Returns an error if ProxyURL is malformed when a proxy mode requires one.
func NewTransport(cfg ProviderConfig) (*Transport, error) {
	if (cfg.ProxyMode == ProxyReverseProxy || cfg.ProxyMode == ProxyCustom) && cfg.ProxyURL != "" {
		if _, err := url.Parse(cfg.ProxyURL); err != nil {
			return nil, fmt.Errorf("provider: invalid proxy URL %q: %w", cfg.ProxyURL, err)
		}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = defaultMaxIdleConns
	}

	httpTransport := &http.Transport{
		TLSClientConfig: cfg.TLSConfig,
		MaxIdleConns:    maxIdleConns,
	}

	codes := cfg.RetryConfig.RetryableErrors
	if len(codes) == 0 {
		codes = defaultRetryableCodes
	}
	retryable := make(map[int]bool, len(codes))
	for _, c := range codes {
		retryable[c] = true
	}

	return &Transport{
		client: &http.Client{
			Timeout:   timeout,
			Transport: httpTransport,
		},
		cfg:       cfg,
		retryable: retryable,
	}, nil
}

// Do executes req with retry and exponential-backoff logic.
// body is the raw request body reused across retry attempts; pass nil for bodyless requests.
// Context cancellation is respected between retries.
func (t *Transport) Do(ctx context.Context, req *http.Request, body []byte) (*http.Response, error) {
	rc := t.cfg.RetryConfig

	maxRetries := rc.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}
	initial := rc.InitialBackoff
	if initial == 0 {
		initial = defaultInitialBackoff
	}
	maxBackoff := rc.MaxBackoff
	if maxBackoff == 0 {
		maxBackoff = defaultMaxBackoff
	}
	multiplier := rc.Multiplier
	if multiplier == 0 {
		multiplier = defaultMultiplier
	}

	backoff := initial
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleep := jitteredBackoff(backoff, rc.Jitter)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}
			next := time.Duration(float64(backoff) * multiplier)
			if next > maxBackoff {
				next = maxBackoff
			}
			backoff = next
		}

		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}

		resp, err := t.client.Do(req.WithContext(ctx))
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}

		if !t.retryable[resp.StatusCode] {
			return resp, nil
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("provider: HTTP %d after attempt %d", resp.StatusCode, attempt+1)
	}

	return nil, lastErr
}

// RewriteURL returns the effective URL for a provider request, applying the configured proxy mode.
//
// ProxyDirect (or no proxy): providerURL is returned unchanged.
// ProxyReverseProxy: scheme and host are replaced with the configured ProxyURL; the provider
// path is appended to the proxy base path, enabling reverse-proxy ad-blocker bypass.
// ProxyCustom: ProxyURL is returned as the full replacement URL.
func (t *Transport) RewriteURL(providerURL string) (string, error) {
	switch t.cfg.ProxyMode {
	case ProxyDirect, "":
		return providerURL, nil

	case ProxyReverseProxy:
		if t.cfg.ProxyURL == "" {
			return providerURL, nil
		}
		base, err := url.Parse(t.cfg.ProxyURL)
		if err != nil {
			return "", fmt.Errorf("provider: invalid proxy URL: %w", err)
		}
		target, err := url.Parse(providerURL)
		if err != nil {
			return "", fmt.Errorf("provider: invalid provider URL: %w", err)
		}
		target.Scheme = base.Scheme
		target.Host = base.Host
		if base.Path != "" && base.Path != "/" {
			basePath := strings.TrimRight(base.Path, "/")
			provPath := strings.TrimLeft(target.Path, "/")
			target.Path = basePath + "/" + provPath
		}
		return target.String(), nil

	case ProxyCustom:
		if t.cfg.ProxyURL == "" {
			return providerURL, nil
		}
		return t.cfg.ProxyURL, nil

	default:
		return providerURL, nil
	}
}

// ResolveSecret resolves the API key from apiKey according to st.
//
// SecretInline (or empty st): returned as-is.
// SecretEnvVar: apiKey may be "${VAR_NAME}" or a plain variable name; resolved from the environment.
// SecretFile: apiKey is a file path; file contents are trimmed and returned.
// SecretVault: not implemented in Phase 1.
func ResolveSecret(apiKey string, st SecretType) (string, error) {
	switch st {
	case SecretInline, "":
		return apiKey, nil

	case SecretEnvVar:
		varName := apiKey
		if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
			varName = apiKey[2 : len(apiKey)-1]
		}
		val := os.Getenv(varName)
		if val == "" {
			return "", fmt.Errorf("provider: env var %q is not set", varName)
		}
		return val, nil

	case SecretFile:
		data, err := os.ReadFile(apiKey)
		if err != nil {
			return "", fmt.Errorf("provider: reading secret file %q: %w", apiKey, err)
		}
		return strings.TrimSpace(string(data)), nil

	case SecretVault:
		return "", fmt.Errorf("provider: vault secret type is not implemented")

	default:
		return "", fmt.Errorf("provider: unknown secret type %q", st)
	}
}

// jitteredBackoff returns d unchanged when jitter is disabled, or a value in [d/2, d) when enabled.
// Equal-jitter ensures a minimum wait of d/2 while preventing thundering-herd synchronisation.
func jitteredBackoff(d time.Duration, jitter bool) time.Duration {
	if !jitter || d <= 0 {
		return d
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}
