package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

func (c *Client) login(ctx context.Context) error {
	_, err := c.request(ctx, "auth/login", func(ctx context.Context) (*http.Response, error) {
		return c.api.LoginWithFormdataBody(ctx, openapi.LoginFormdataRequestBody{Username: c.cfg.Username, Password: c.cfg.Password})
	})
	c.loggedIn = err == nil
	return err
}
func (c *Client) CheckVersion(ctx context.Context) error {
	for _, v := range []struct {
		endpoint, want string
		call           operation
	}{
		{"app/version", "v5.3.0", func(ctx context.Context) (*http.Response, error) { return c.api.GetVersion(ctx) }},
		{"app/webapiVersion", "2.16.2", func(ctx context.Context) (*http.Response, error) { return c.api.GetWebAPIVersion(ctx) }},
	} {
		b, err := c.do(ctx, v.endpoint, true, v.call)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(b)) != v.want {
			return fmt.Errorf("unsupported qBittorrent version: %s must report %s; supported baseline is 5.3.0 / Web API 2.16.2", v.endpoint, v.want)
		}
	}
	return nil
}
