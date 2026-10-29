package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

// Preferences is an allowlist. Never decode or return server credential fields.
// Nil pointers omit unmanaged keys; pointers to false/zero are explicit updates.
type Preferences struct {
	SavePath         *string `json:"save_path,omitempty"`
	TempPath         *string `json:"temp_path,omitempty"`
	TempPathEnabled  *bool   `json:"temp_path_enabled,omitempty"`
	AutoTMMEnabled   *bool   `json:"auto_tmm_enabled,omitempty"`
	DownloadLimit    *int64  `json:"dl_limit,omitempty"`
	UploadLimit      *int64  `json:"up_limit,omitempty"`
	AltDownloadLimit *int64  `json:"alt_dl_limit,omitempty"`
	AltUploadLimit   *int64  `json:"alt_up_limit,omitempty"`
	DHT              *bool   `json:"dht,omitempty"`
	PeX              *bool   `json:"pex,omitempty"`
	LSD              *bool   `json:"lsd,omitempty"`
	ListenPort       *int64  `json:"listen_port,omitempty"`
	UPnP             *bool   `json:"upnp,omitempty"`
}

func decodeObject(b []byte, out any) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(b, &object); err != nil || object == nil {
		return errors.New("qBittorrent returned an invalid JSON object")
	}
	if err := json.Unmarshal(b, out); err != nil {
		return errors.New("qBittorrent JSON does not match the supported schema")
	}
	return nil
}
func (c *Client) Preferences(ctx context.Context) (Preferences, error) {
	var p Preferences
	b, err := c.do(ctx, http.MethodGet, "app/preferences", nil)
	if err != nil {
		return p, err
	}
	err = decodeObject(b, &p)
	return p, err
}
func (c *Client) SetPreferences(ctx context.Context, p Preferences) error {
	b, err := json.Marshal(p)
	if err != nil {
		return errors.New("cannot encode preferences")
	}
	_, err = c.do(ctx, http.MethodPost, "app/setPreferences", url.Values{"json": {string(b)}})
	return err
}
