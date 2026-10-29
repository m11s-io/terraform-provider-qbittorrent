package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) login(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, "auth/login", url.Values{"username": {c.cfg.Username}, "password": {c.cfg.Password}})
	c.loggedIn = err == nil
	return err
}
func (c *Client) CheckVersion(ctx context.Context) error {
	for _, v := range []struct{ endpoint, want string }{{"app/version", "v5.3.0"}, {"app/webapiVersion", "2.16.2"}} {
		b, err := c.do(ctx, http.MethodGet, v.endpoint, nil)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(b)) != v.want {
			return fmt.Errorf("unsupported qBittorrent version: %s must report %s; supported baseline is 5.3.0 / Web API 2.16.2", v.endpoint, v.want)
		}
	}
	return nil
}
