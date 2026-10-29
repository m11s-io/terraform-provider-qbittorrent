// Package client implements the qBittorrent 5.3.0 Web API subset used by this provider.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

type Config struct {
	URL, APIKey, Username, Password string
	Timeout                         time.Duration
}
type Client struct {
	base     *url.URL
	http     *http.Client
	api      *openapi.Client
	cfg      Config
	mu       sync.Mutex
	loggedIn bool
}
type HTTPError struct {
	StatusCode int
	Endpoint   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("qBittorrent %s returned HTTP %d (check authentication, permissions and server logs)", e.Endpoint, e.StatusCode)
}

func New(cfg Config) (*Client, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("url must be an absolute HTTP(S) WebUI URL without credentials, query or fragment")
	}
	if cfg.APIKey != "" {
		if cfg.Username != "" || cfg.Password != "" {
			return nil, errors.New("api_key cannot be combined with username/password")
		}
	} else if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("provide api_key or both username and password")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	u.RawPath = ""
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &Client{base: u, cfg: cfg, http: &http.Client{Timeout: cfg.Timeout, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	c.api, err = openapi.NewClient(u.String()+"api/v2/", openapi.WithHTTPClient(c.http), openapi.WithRequestEditorFn(c.prepareRequest))
	if err != nil {
		return nil, errors.New("cannot construct qBittorrent API client")
	}
	return c, nil
}

// Generated operations own URL paths, HTTP methods and form serialization.
// The adapter retains authentication policy and bounded, sanitized responses.
type operation func(context.Context) (*http.Response, error)

func (c *Client) prepareRequest(_ context.Context, req *http.Request) error {
	req.Header.Set("Origin", c.base.Scheme+"://"+c.base.Host)
	req.Header.Set("Referer", c.base.String())
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	return nil
}

// request never follows redirects or includes response bodies in diagnostics.
func (c *Client) request(ctx context.Context, endpoint string, call operation) ([]byte, error) {
	resp, err := call(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, fmt.Errorf("qBittorrent %s timed out", endpoint)
		}
		return nil, fmt.Errorf("qBittorrent %s transport failed (check connectivity and TLS)", endpoint)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Endpoint: endpoint}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("qBittorrent %s response read failed", endpoint)
	}
	if len(b) > 4*1024*1024 {
		return nil, errors.New("qBittorrent response exceeds 4 MiB")
	}
	return b, nil
}
func (c *Client) do(ctx context.Context, endpoint string, readOnly bool, call operation) ([]byte, error) {
	if c.cfg.APIKey != "" {
		return c.request(ctx, endpoint, call)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}
	b, err := c.request(ctx, endpoint, call)
	var api *HTTPError
	if errors.As(err, &api) && api.StatusCode == 403 {
		c.loggedIn = false
		if readOnly {
			if err = c.login(ctx); err != nil {
				return nil, err
			}
			b, err = c.request(ctx, endpoint, call)
			if errors.As(err, &api) && api.StatusCode == 403 {
				c.loggedIn = false
			}
		}
	}
	return b, err
}
