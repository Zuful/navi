// Package httpclient provides a shared HTTP client with rate limiting and caching.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client wraps net/http.Client with rate limiting and caching.
type Client struct {
	http  *http.Client
	cache *Cache

	// Token bucket rate limiter.
	mu       sync.Mutex
	tokens   float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// Option configures a Client.
type Option func(*Client)

// WithCache attaches a cache to the client.
func WithCache(c *Cache) Option {
	return func(cl *Client) { cl.cache = c }
}

// WithRateLimit sets the rate limiter parameters.
func WithRateLimit(requestsPerSecond float64, burst int) Option {
	return func(cl *Client) {
		cl.maxTokens = float64(burst)
		cl.tokens = float64(burst)
		cl.refillRate = requestsPerSecond
		cl.lastRefill = time.Now()
	}
}

// New creates a new rate-limited HTTP client.
func New(opts ...Option) *Client {
	c := &Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// Default rate limit: 10 req/s, burst of 20.
		maxTokens:  20,
		tokens:     20,
		refillRate: 10,
		lastRefill: time.Now(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Do executes an HTTP request with rate limiting. If the request is a GET
// and caching is enabled, it checks/populates the cache.
func (c *Client) Do(ctx context.Context, req *http.Request) ([]byte, error) {
	cacheKey := req.Method + " " + req.URL.String()

	// Check cache for GET requests.
	if req.Method == http.MethodGet && c.cache != nil {
		if data, ok := c.cache.Get(cacheKey); ok {
			return data, nil
		}
	}

	// Wait for rate limit token.
	if err := c.waitForToken(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	resp, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Cache GET responses.
	if req.Method == http.MethodGet && c.cache != nil {
		c.cache.Set(cacheKey, body)
	}

	return body, nil
}

// waitForToken blocks until a rate limit token is available.
func (c *Client) waitForToken(ctx context.Context) error {
	for {
		c.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(c.lastRefill).Seconds()
		c.tokens += elapsed * c.refillRate
		if c.tokens > c.maxTokens {
			c.tokens = c.maxTokens
		}
		c.lastRefill = now

		if c.tokens >= 1 {
			c.tokens--
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()

		// Wait a short interval before retrying.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
