package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestWebUIPartialUpdatesAndDefaults(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("clientdata must use POST")
		}
		r.ParseForm()
		switch r.URL.Path {
		case "/proxy/api/v2/clientdata/store":
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("data")), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 3 || got["qbt_color_scheme"] != nil || got["qbt_display_density"] != nil || got["qbt_use_virtual_list"] != false {
				t.Fatalf("wrong patch: %v", got)
			}
		case "/proxy/api/v2/clientdata/load":
			var keys []string
			if err := json.Unmarshal([]byte(r.Form.Get("keys")), &keys); err != nil || len(keys) != 11 {
				t.Fatalf("wrong load filter: %v %v", keys, err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"qbt_use_virtual_list":false,"qbt_color_scheme":"dark","unmanaged":"secret"}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}, false)
	if err := c.SetWebUIPreferences(context.Background(), map[string]any{"color_scheme": "auto", "display_density": "default", "use_virtual_list": false}); err != nil {
		t.Fatal(err)
	}
	got, err := c.WebUIPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["color_scheme"] != "dark" || got["display_density"] != "default" || got["dblclick_download"] != "1" || got["use_virtual_list"] != false || len(got) != 11 {
		t.Fatalf("wrong defaults/allowlist: %v", got)
	}
}
