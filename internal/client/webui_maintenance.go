package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

var webUIEndpointKeys = map[string]bool{
	"web_ui_address": true, "web_ui_port": true, "use_https": true,
	"web_ui_https_cert_path": true, "web_ui_https_key_path": true,
}

var webUIAuthKeys = map[string]bool{"web_ui_username": true, "web_ui_password": true}

// SetWebUIMaintenancePatch sends one narrow write to qBittorrent. This method
// never retries: callers must treat transport failures as an uncertain outcome.
func (c *Client) SetWebUIMaintenancePatch(ctx context.Context, values map[string]any, auth bool) error {
	allowed := webUIEndpointKeys
	if auth {
		allowed = webUIAuthKeys
	}
	if len(values) == 0 {
		return nil
	}
	for key := range values {
		if !allowed[key] {
			return errors.New("unsupported WebUI maintenance field")
		}
	}
	b, err := json.Marshal(values)
	if err != nil {
		return errors.New("cannot encode WebUI maintenance patch")
	}
	_, err = c.do(ctx, "app/setPreferences", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.SetPreferencesWithFormdataBody(ctx, openapi.SetPreferencesFormdataRequestBody{Json: string(b)})
	})
	return err
}

// MaintenanceCandidate returns a separate client with the selected endpoint
// and credentials. The original client's cookies and request policy are not
// modified, so other resources can continue using it safely.
func (c *Client) MaintenanceCandidate(targetURL, username, password string) (*Client, error) {
	cfg := c.cfg
	if targetURL != "" {
		cfg.URL = targetURL
	}
	if username != "" || password != "" {
		if cfg.APIKey != "" {
			return nil, errors.New("cannot change WebUI username/password while using Bearer authentication")
		}
		if username == "" || password == "" {
			return nil, errors.New("both candidate username and password are required")
		}
		cfg.Username, cfg.Password = username, password
	}
	return New(cfg)
}

func (c *Client) UsesBearerAuthentication() bool { return c.cfg.APIKey != "" }

// VerifyWebUIEndpoint reads only the nonsecret listener fields needed to
// confirm a reconnect. It deliberately does not decode or expose other values.
func (c *Client) VerifyWebUIEndpoint(ctx context.Context, expected map[string]any) error {
	b, err := c.do(ctx, "app/preferences", true, func(ctx context.Context) (*http.Response, error) {
		return c.api.GetPreferences(ctx)
	})
	if err != nil {
		return err
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		return errors.New("qBittorrent returned invalid preferences during reconnect verification")
	}
	for key, want := range expected {
		raw, ok := got[key]
		if !ok {
			return fmt.Errorf("qBittorrent reconnect verification did not return %s", key)
		}
		wantJSON, _ := json.Marshal(want)
		var actual, wanted any
		if json.Unmarshal(raw, &actual) != nil || json.Unmarshal(wantJSON, &wanted) != nil || fmt.Sprint(actual) != fmt.Sprint(wanted) {
			return fmt.Errorf("qBittorrent reconnect verification did not match %s", key)
		}
	}
	return nil
}
