package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

// Preferences is an allowlist. Never decode or return server credential fields.
// Nil pointers omit unmanaged keys; pointers to false/zero are explicit updates.
type Preferences = openapi.Preferences

// PreferencesPatch is the separate typed write contract. It can include
// explicitly supplied write-only values without adding them to GET models.
type PreferencesPatch = openapi.PreferencesPatch

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
	b, err := c.do(ctx, "app/preferences", true, func(ctx context.Context) (*http.Response, error) {
		return c.api.GetPreferences(ctx)
	})
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
	// Translate the backward-compatible read model into the write allowlist.
	// This discards computed outputs if a future schema composes them into GETs.
	var patch PreferencesPatch
	if err := json.Unmarshal(b, &patch); err != nil {
		return errors.New("cannot encode preferences")
	}
	return c.SetPreferencesPatch(ctx, patch)
}

func (c *Client) SetPreferencesPatch(ctx context.Context, p PreferencesPatch) error {
	b, err := json.Marshal(p)
	if err != nil {
		return errors.New("cannot encode preferences")
	}
	_, err = c.do(ctx, "app/setPreferences", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.SetPreferencesWithFormdataBody(ctx, openapi.SetPreferencesFormdataRequestBody{Json: string(b)})
	})
	return err
}
