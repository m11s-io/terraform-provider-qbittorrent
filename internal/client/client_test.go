package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc, session bool) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	cfg := Config{URL: s.URL + "/proxy", APIKey: "secret-key", Timeout: time.Second}
	if session {
		cfg.APIKey = ""
		cfg.Username = "admin"
		cfg.Password = "secret-password"
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func versionHandler(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/proxy/api/v2/app/version":
		fmt.Fprint(w, "v5.3.0")
		return true
	case "/proxy/api/v2/app/webapiVersion":
		fmt.Fprint(w, "2.16.2")
		return true
	}
	return false
}
func TestBearerAndPartialPreferences(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Error("missing bearer")
		}
		if r.Header.Get("Origin") != "http://"+r.Host || r.Header.Get("Referer") != "http://"+r.Host+"/proxy/" {
			t.Error("wrong same-origin headers")
		}
		if versionHandler(w, r) {
			return
		}
		switch r.URL.Path {
		case "/proxy/api/v2/app/setPreferences":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Error("wrong write encoding")
			}
			r.ParseForm()
			var got map[string]any
			json.Unmarshal([]byte(r.Form.Get("json")), &got)
			if len(got) != 2 || got["dht"] != false || got["dl_limit"] != float64(0) {
				t.Errorf("wrong partial update: %v", got)
			}
		case "/proxy/api/v2/app/preferences":
			fmt.Fprint(w, `{"dht":false,"dl_limit":0,"save_path":"/downloads","web_ui_api_key":"must-not-be-returned"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}, false)
	if err := c.CheckVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	f, z := false, int64(0)
	if err := c.SetPreferences(context.Background(), Preferences{DHT: &f, DownloadLimit: &z}); err != nil {
		t.Fatal(err)
	}
	p, err := c.Preferences(context.Background())
	if err != nil || p.SavePath == nil || *p.SavePath != "/downloads" {
		t.Fatalf("%+v %v", p, err)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "must-not") || strings.Contains(string(b), "api_key") {
		t.Fatal("secret escaped allowlist")
	}
}
func TestLoginCookieAndBoundedRecovery(t *testing.T) {
	var logins, reads atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "auth/login") {
			logins.Add(1)
			r.ParseForm()
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret-password" {
				t.Error("wrong credentials")
			}
			http.SetCookie(w, &http.Cookie{Name: "QBT_SID_abc", Value: "cookie", Path: "/"})
			w.WriteHeader(204)
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("session sent bearer")
		}
		if _, err := r.Cookie("QBT_SID_abc"); err != nil {
			w.WriteHeader(403)
			return
		}
		if reads.Add(1) == 1 {
			w.WriteHeader(403)
			return
		}
		if !versionHandler(w, r) {
			fmt.Fprint(w, `{"dht":true}`)
		}
	}, true)
	if _, err := c.Preferences(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 || reads.Load() != 2 {
		t.Fatalf("logins=%d reads=%d", logins.Load(), reads.Load())
	}
}
func TestAuthenticationFailuresAndNoWriteReplay(t *testing.T) {
	for _, status := range []int{401, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var n atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				n.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, "secret-password")
			}, true)
			_, err := c.Preferences(context.Background())
			if err == nil || strings.Contains(err.Error(), "secret-password") {
				t.Fatalf("unsafe error %v", err)
			}
			if n.Load() != 1 {
				t.Fatal("unbounded login")
			}
		})
	}
	var writes atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "auth/login") {
			w.WriteHeader(204)
			return
		}
		writes.Add(1)
		w.WriteHeader(403)
	}, true)
	if err := c.SetPreferences(context.Background(), Preferences{}); err == nil {
		t.Fatal("missing error")
	}
	if writes.Load() != 1 {
		t.Fatal("mutation replayed")
	}
	c = newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "auth/login") {
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(403)
	}, true)
	if _, err := c.Preferences(context.Background()); err == nil {
		t.Fatal("persistent expired session accepted")
	}
}
func TestMalformedErrorsCancellationTimeout(t *testing.T) {
	for _, body := range []string{`no`, `null`, `[]`, `{"dht":"wrong"}`, `{} trailing`} {
		t.Run(body, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }, false)
			if _, err := c.Preferences(context.Background()); err == nil {
				t.Fatal("invalid preferences accepted")
			}
		})
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Preferences(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	c.http.Timeout = 20 * time.Millisecond
	if _, err := c.Preferences(context.Background()); err == nil {
		t.Fatal("missing timeout")
	}
}
func TestVersionAndRedirectSafety(t *testing.T) {
	for _, v := range []string{"v5.2.0", "v5.3.1"} {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, v) }, false)
		if err := c.CheckVersion(context.Background()); err == nil {
			t.Fatal("unsupported version")
		}
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "webapiVersion") {
			fmt.Fprint(w, "2.15.0")
		} else {
			fmt.Fprint(w, "v5.3.0")
		}
	}, false)
	if err := c.CheckVersion(context.Background()); err == nil {
		t.Fatal("unsupported API")
	}
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Store(true) }))
	defer target.Close()
	c = newTestClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }, false)
	if _, err := c.Preferences(context.Background()); err == nil || reached.Load() {
		t.Fatal("redirect followed")
	}
}
func TestConfigValidation(t *testing.T) {
	for _, cfg := range []Config{{URL: "relative", APIKey: "k"}, {URL: "ftp://host", APIKey: "k"}, {URL: "https://u:p@host", APIKey: "k"}, {URL: "https://host?secret=x", APIKey: "k"}, {URL: "https://host#x", APIKey: "k"}, {URL: "https://host"}, {URL: "https://host", Username: "a"}, {URL: "https://host", APIKey: "k", Password: "p"}} {
		if _, err := New(cfg); err == nil {
			t.Errorf("invalid config accepted: URL %q", cfg.URL)
		}
	}
}
func TestCategoriesContract(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/categories") {
			fmt.Fprint(w, `{"movies":{"name":"movies","savePath":"/data"}}`)
			return
		}
		if r.Method != "POST" {
			t.Error("wrong method")
		}
		r.ParseForm()
		if strings.HasSuffix(r.URL.Path, "removeCategories") {
			if r.Form.Get("categories") != "movies" {
				t.Error("wrong delete")
			}
			return
		}
		if r.Form.Get("category") != "movies" || r.Form.Get("savePath") != "/data" {
			t.Errorf("wrong category body %v", r.Form)
		}
	}, false)
	ctx := context.Background()
	if err := c.CreateCategory(ctx, Category{Name: "movies", SavePath: "/data"}); err != nil {
		t.Fatal(err)
	}
	if err := c.EditCategory(ctx, Category{Name: "movies", SavePath: "/data"}); err != nil {
		t.Fatal(err)
	}
	cats, err := c.Categories(ctx)
	if err != nil || cats["movies"].SavePath != "/data" {
		t.Fatalf("%v %v", cats, err)
	}
	if err := c.DeleteCategory(ctx, "movies"); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{404, 409} {
		c = newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }, false)
		err := c.CreateCategory(ctx, Category{Name: "movies"})
		var api *HTTPError
		if !errors.As(err, &api) || api.StatusCode != code {
			t.Fatalf("lost status: %v", err)
		}
	}
}
