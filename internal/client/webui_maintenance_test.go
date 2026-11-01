package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWebUIAuthTransitionUsesSeparateCandidateCredentials(t *testing.T) {
	var password = "old-password"
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v2/auth/login"):
			_ = r.ParseForm()
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != password {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: password, Path: "/"})
			_, _ = io.WriteString(w, "Ok.")
		case strings.HasSuffix(r.URL.Path, "/api/v2/app/setPreferences"):
			writes.Add(1)
			password = "new-password"
			_, _ = io.WriteString(w, "Ok.")
		case strings.HasSuffix(r.URL.Path, "/api/v2/app/preferences"):
			cookie, err := r.Cookie("SID")
			if err != nil || cookie.Value != password {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, err := New(Config{URL: server.URL, Username: "admin", Password: "old-password"})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetWebUIMaintenancePatch(context.Background(), map[string]any{"web_ui_password": "new-password"}, true); err != nil {
		t.Fatalf("write password: %v", err)
	}
	candidate, err := c.MaintenanceCandidate("", "admin", "new-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = candidate.VerifyWebUIEndpoint(context.Background(), map[string]any{}); err != nil {
		t.Fatalf("re-authenticate candidate: %v", err)
	}
	if writes.Load() != 1 {
		t.Fatalf("password mutation count = %d, want 1", writes.Load())
	}
	if c.cfg.Password != "old-password" {
		t.Fatal("maintenance transition mutated shared client credentials")
	}
}

func TestWebUIEndpointLostResponseReconnectsWithoutReplay(t *testing.T) {
	var writes atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/v2/auth/login") {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "target", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/api/v2/app/preferences") {
			_, _ = fmt.Fprint(w, `{"web_ui_port":47000}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer target.Close()
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/v2/auth/login") {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "old", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/api/v2/app/setPreferences") {
			writes.Add(1)
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = buf.Flush()
				_ = conn.Close()
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer old.Close()
	c, err := New(Config{URL: old.URL, Username: "admin", Password: "old-password"})
	if err != nil {
		t.Fatal(err)
	}
	err = c.SetWebUIMaintenancePatch(context.Background(), map[string]any{"web_ui_port": int64(47000)}, false)
	if err == nil {
		t.Fatal("expected deliberately lost initial response")
	}
	candidate, err := c.MaintenanceCandidate(target.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = candidate.VerifyWebUIEndpoint(context.Background(), map[string]any{"web_ui_port": int64(47000)}); err != nil {
		t.Fatalf("candidate reconnect: %v", err)
	}
	if got := writes.Load(); got != 1 {
		t.Fatalf("mutation count = %d, want exactly one", got)
	}
}

func TestMaintenanceCandidatePreservesBearerAndRejectsSelfTransition(t *testing.T) {
	c, err := New(Config{URL: "http://127.0.0.1:12345", APIKey: "fixture-key"})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := c.MaintenanceCandidate("http://127.0.0.1:23456", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.UsesBearerAuthentication() {
		t.Fatal("candidate lost Bearer authentication")
	}
	if _, err = c.MaintenanceCandidate("http://127.0.0.1:23456", "new-user", "new-password"); err == nil {
		t.Fatal("expected unsupported Bearer self-transition")
	}
}

func TestMaintenanceCandidateKeepsTLSVerification(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"web_ui_port":47000}`) }))
	defer tlsServer.Close()
	c, err := New(Config{URL: "http://127.0.0.1:12345", APIKey: "fixture-key"})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := c.MaintenanceCandidate(tlsServer.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	err = candidate.VerifyWebUIEndpoint(context.Background(), map[string]any{"web_ui_port": int64(47000)})
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("expected verified-TLS failure, got %v", err)
	}
}
