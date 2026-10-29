package openapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

// Incorrect form encoding would split ampersands/pluses into different values,
// or lose explicit false/zero in the JSON string carried by the form.
func TestGeneratedForms(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected request encoding: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch r.URL.Path {
		case "/proxy/api/v2/auth/login":
			if r.PostForm.Get("username") != "a+b&雪" || r.PostForm.Get("password") != "p=+&%\n" || len(r.PostForm) != 2 {
				t.Error("login fields did not round-trip")
			}
		case "/proxy/api/v2/app/setPreferences":
			if len(r.PostForm) != 1 || r.PostForm.Get("json") != `{"dht":false,"dl_limit":0}` {
				t.Errorf("unexpected partial preferences: %v", r.PostForm)
			}
		case "/proxy/api/v2/torrents/createCategory":
			if r.PostForm.Get("category") != "a+b&雪" || !r.PostForm.Has("savePath") || r.PostForm.Get("savePath") != "" || len(r.PostForm) != 2 {
				t.Error("category fields did not round-trip")
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()
	c, err := openapi.NewClient(s.URL + "/proxy/api/v2/")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	check := func(resp *http.Response, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("unexpected response: %d", resp.StatusCode)
		}
	}
	check(c.LoginWithFormdataBody(ctx, openapi.LoginFormdataRequestBody{Username: "a+b&雪", Password: "p=+&%\n"}))
	f, z := false, int64(0)
	b, err := json.Marshal(openapi.Preferences{DHT: &f, DownloadLimit: &z})
	if err != nil {
		t.Fatal(err)
	}
	check(c.SetPreferencesWithFormdataBody(ctx, openapi.SetPreferencesFormdataRequestBody{Json: string(b)}))
	empty := ""
	check(c.CreateCategoryWithFormdataBody(ctx, openapi.CreateCategoryFormdataRequestBody{Category: "a+b&雪", SavePath: &empty}))
}

func TestPreferencesAllowlist(t *testing.T) {
	// This fixture pins upstream field names independently of the schema and
	// catches dropped false/zero values or accidental capture of secret keys.
	const response = `{"save_path":"/downloads","temp_path":"","temp_path_enabled":false,"auto_tmm_enabled":false,"dl_limit":0,"up_limit":1024,"alt_dl_limit":2048,"alt_up_limit":0,"dht":false,"pex":false,"lsd":false,"listen_port":6881,"upnp":false,"web_ui_api_key":"secret"}`
	var p openapi.Preferences
	if err := json.Unmarshal([]byte(response), &p); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"save_path": "/downloads", "temp_path": "", "temp_path_enabled": false,
		"auto_tmm_enabled": false, "dl_limit": float64(0), "up_limit": float64(1024),
		"alt_dl_limit": float64(2048), "alt_up_limit": float64(0), "dht": false,
		"pex": false, "lsd": false, "listen_port": float64(6881), "upnp": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preference keys/values or allowlist changed: got %v", got)
	}
}
