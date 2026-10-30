package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

// These are the effective defaults used by the 5.3.0 WebUI when a key is absent.
// Only this allowlist is ever requested, returned or patched by the provider.
func webUIDefaults() map[string]any {
	return map[string]any{
		"date_format": "default", "color_scheme": "auto", "display_density": "default",
		"use_alt_row_colors": false, "dblclick_download": "1", "dblclick_complete": "1", "dblclick_filter": "1",
		"hide_zero_status_filters": false, "add_new_torrent_dialog_enabled": true,
		"full_url_tracker_column": false, "use_virtual_list": true,
	}
}

func (c *Client) WebUIPreferences(ctx context.Context) (map[string]any, error) {
	values := webUIDefaults()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, "qbt_"+key)
	}
	encoded, _ := json.Marshal(keys)
	b, err := c.do(ctx, "clientdata/load", true, func(ctx context.Context) (*http.Response, error) {
		return c.api.LoadClientDataWithFormdataBody(ctx, openapi.LoadClientDataFormdataRequestBody{Keys: string(encoded)})
	})
	if err != nil {
		return nil, err
	}
	var p openapi.WebUIPreferences
	if err = decodeObject(b, &p); err != nil {
		return nil, err
	}
	b, _ = json.Marshal(p)
	var remote map[string]any
	json.Unmarshal(b, &remote)
	for k, v := range remote {
		values[strings.TrimPrefix(k, "qbt_")] = v
	}
	return values, nil
}

func (c *Client) SetWebUIPreferences(ctx context.Context, patch map[string]any) error {
	allowed := webUIDefaults()
	wire := map[string]any{}
	for k, v := range patch {
		if _, ok := allowed[k]; !ok {
			return errors.New("unsupported WebUI preference")
		}
		// The native WebUI removes these overrides to restore automatic behavior.
		if (k == "color_scheme" && v == "auto") || (k == "display_density" && v == "default") {
			v = nil
		}
		wire["qbt_"+k] = v
	}
	b, err := json.Marshal(wire)
	if err != nil {
		return errors.New("cannot encode WebUI preferences")
	}
	_, err = c.do(ctx, "clientdata/store", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.StoreClientDataWithFormdataBody(ctx, openapi.StoreClientDataFormdataRequestBody{Data: string(b)})
	})
	return err
}
