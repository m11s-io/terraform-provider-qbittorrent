package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	testconfig "github.com/hashicorp/terraform-plugin-testing/config"
	resourceTesting "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type webUIMaintenanceTestProvider struct{ provider.Provider }

func (p *webUIMaintenanceTestProvider) Resources(ctx context.Context) []func() resource.Resource {
	resources := p.Provider.Resources(ctx)
	hasEndpoint, hasAuth := false, false
	for _, factory := range resources {
		switch factory().(type) {
		case *webuiEndpointResource:
			hasEndpoint = true
		case *webuiAuthResource:
			hasAuth = true
		}
	}
	if !hasEndpoint {
		resources = append(resources, func() resource.Resource { return &webuiEndpointResource{} })
	}
	if !hasAuth {
		resources = append(resources, func() resource.Resource { return &webuiAuthResource{} })
	}
	return resources
}

func webUIMaintenanceTestFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"qbittorrent": providerserver.NewProtocol6WithError(&webUIMaintenanceTestProvider{Provider: New("test")()})}
}

// TestAccWebUIEndpointLostResponseRecovery is an opt-in Terraform CLI lifecycle
// test using a stateful local Web API fixture. It models an actual listener
// handoff: the old listener closes while the mutation response is in flight,
// the target listener starts before the recovery plan, and the mutation count
// remains one across apply, refresh, and the follow-up no-op apply.
func TestAccWebUIEndpointLostResponseRecovery(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run the WebUI maintenance Terraform CLI fixture")
	}
	t.Setenv("TF_ACC_PROVIDER_NAMESPACE", "m11s-io")
	t.Setenv("TF_ACC_PROVIDER_HOST", "registry.terraform.io")

	targetListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	targetAddr := targetListener.Addr().String()
	_ = targetListener.Close()
	targetURL := "http://" + targetAddr
	var targetPort int
	if _, err := fmt.Sscanf(strings.Split(targetAddr, ":")[1], "%d", &targetPort); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	var endpointWrites atomic.Int32
	var authWrites atomic.Int32
	var authPasswordWrites atomic.Int32
	var oldPort atomic.Int32
	var targetPreferencesReads atomic.Int32
	var currentUsername, currentPassword atomic.Value
	currentUsername.Store("admin")
	currentPassword.Store("fixture-current")
	oldPort.Store(18080)
	var old *httptest.Server
	var target *httptest.Server
	var startTarget func()
	serve := func(target bool) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/api/v2/auth/login"):
				_ = r.ParseForm()
				if r.Form.Get("username") != currentUsername.Load().(string) || r.Form.Get("password") != currentPassword.Load().(string) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "SID", Value: currentPassword.Load().(string), Path: "/"})
				_, _ = io.WriteString(w, "Ok.")
			case strings.HasSuffix(r.URL.Path, "/api/v2/app/version"):
				_, _ = io.WriteString(w, "v5.3.0")
			case strings.HasSuffix(r.URL.Path, "/api/v2/app/webapiVersion"):
				_, _ = io.WriteString(w, "2.16.2")
			case strings.HasSuffix(r.URL.Path, "/api/v2/app/preferences"):
				cookie, err := r.Cookie("SID")
				if err != nil || cookie.Value != currentPassword.Load().(string) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				if target && targetPreferencesReads.Add(1) == 1 {
					http.Error(w, "target is not ready", http.StatusServiceUnavailable)
					return
				}
				p := oldPort.Load()
				if target {
					p = int32(targetPort)
				}
				_, _ = fmt.Fprintf(w, `{"web_ui_address":"127.0.0.1","web_ui_port":%d,"use_https":false,"web_ui_https_cert_path":"/tmp/cert","web_ui_https_key_path":"/tmp/key","web_ui_username":%q}`, p, currentUsername.Load().(string))
			case strings.HasSuffix(r.URL.Path, "/api/v2/app/setPreferences"):
				cookie, err := r.Cookie("SID")
				if err != nil || cookie.Value != currentPassword.Load().(string) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				if target {
					_ = r.ParseForm()
					var patch map[string]any
					if err := json.Unmarshal([]byte(r.Form.Get("json")), &patch); err != nil {
						http.Error(w, "bad patch", 400)
						return
					}
					if username, ok := patch["web_ui_username"].(string); ok {
						currentUsername.Store(username)
						authWrites.Add(1)
						writes.Add(1)
						if password, ok := patch["web_ui_password"].(string); ok {
							currentPassword.Store(password)
							authPasswordWrites.Add(1)
						}
						_ = os.Setenv("QBITTORRENT_USERNAME", username)
						_ = os.Setenv("QBITTORRENT_PASSWORD", currentPassword.Load().(string))
						_, _ = io.WriteString(w, "Ok.")
						return
					}
					if _, ok := patch["web_ui_port"]; ok {
						endpointWrites.Add(1)
						writes.Add(1)
						_, _ = io.WriteString(w, "Ok.")
						return
					}
					http.Error(w, "unsupported fixture patch", 400)
					return
				}
				if err := r.ParseForm(); err != nil {
					http.Error(w, "bad form", 400)
					return
				}
				var patch map[string]any
				if err := json.Unmarshal([]byte(r.Form.Get("json")), &patch); err != nil {
					http.Error(w, "bad patch", 400)
					return
				}
				if int(patch["web_ui_port"].(float64)) != targetPort {
					http.Error(w, "unexpected port", 400)
					return
				}
				writes.Add(1)
				endpointWrites.Add(1)
				oldPort.Store(int32(targetPort))
				// Move the fixture listener and drop the active connection before
				// the provider can read the response.
				_ = old.Listener.Close()
				_ = os.Setenv("QBITTORRENT_URL", targetURL)
				startTarget()
				if h, ok := w.(http.Hijacker); ok {
					conn, buf, e := h.Hijack()
					if e == nil {
						_ = buf.Flush()
						_ = conn.Close()
					}
				}
			default:
				http.NotFound(w, r)
			}
		})
	}
	old = httptest.NewServer(serve(false))
	defer old.Close()
	t.Setenv("QBITTORRENT_URL", old.URL)
	t.Setenv("QBITTORRENT_USERNAME", "admin")
	t.Setenv("QBITTORRENT_PASSWORD", "fixture-current")
	startTarget = func() {
		if target != nil {
			return
		}
		listener, e := net.Listen("tcp", targetAddr)
		if e != nil {
			t.Fatal(e)
		}
		target = httptest.NewUnstartedServer(serve(true))
		target.Listener = listener
		target.Start()
		t.Cleanup(target.Close)
	}

	providerConfig := `terraform {
  required_providers {
    qbittorrent = {
      source = "registry.terraform.io/m11s-io/qbittorrent"
    }
  }
}
provider "qbittorrent" {}
`
	config := providerConfig + fmt.Sprintf(`resource "qbittorrent_webui_endpoint" "listener" {
  web_ui_address = "127.0.0.1"
  web_ui_port = %d
  use_https = false
  post_update_url = %q
}`, targetPort, targetURL)
	oldConfig, targetConfig := config, config
	canary := randomCredentialCanary(t)
	authConfig := targetConfig + fmt.Sprintf(`
variable "target_password" {
  type = string
  sensitive = true
  ephemeral = true
}
resource "qbittorrent_webui_auth" "admin" {
  username = "operator"
  password_wo = var.target_password
  password_wo_version = 1
}
`)
	authVars := testconfig.Variables{"target_password": testconfig.StringVariable(canary)}
	checkNoSecret := func(state *terraform.State) error {
		for _, instance := range state.RootModule().Resources {
			for name, value := range instance.Primary.Attributes {
				if name == "password_wo" && value != "" {
					return fmt.Errorf("password attribute %q entered state", name)
				}
				if strings.Contains(value, canary) {
					return fmt.Errorf("password canary entered state attribute %q", name)
				}
			}
		}
		return nil
	}
	resourceTesting.Test(t, resourceTesting.TestCase{ProtoV6ProviderFactories: webUIMaintenanceTestFactories(), CheckDestroy: func(*terraform.State) error {
		if endpointWrites.Load() != 1 || authWrites.Load() != 2 || authPasswordWrites.Load() != 1 {
			return fmt.Errorf("destroy changed maintenance configuration: endpoint=%d auth=%d password=%d", endpointWrites.Load(), authWrites.Load(), authPasswordWrites.Load())
		}
		return nil
	}, Steps: []resourceTesting.TestStep{
		{Config: oldConfig, Check: resourceTesting.ComposeAggregateTestCheckFunc(resourceTesting.TestCheckResourceAttr("qbittorrent_webui_endpoint.listener", "id", "application"), func(*terraform.State) error {
			if writes.Load() != 1 {
				return fmt.Errorf("initial mutation count=%d, want 1", writes.Load())
			}
			return nil
		})},
		{Config: targetConfig, PreConfig: func() { _ = os.Setenv("QBITTORRENT_URL", targetURL); startTarget() }, PlanOnly: true, Check: resourceTesting.ComposeAggregateTestCheckFunc(resourceTesting.TestCheckResourceAttr("qbittorrent_webui_endpoint.listener", "web_ui_port", fmt.Sprint(targetPort)), func(*terraform.State) error {
			if writes.Load() != 1 {
				return fmt.Errorf("refresh replayed mutation; count=%d", writes.Load())
			}
			return nil
		})},
		{Config: targetConfig, Check: func(*terraform.State) error {
			if writes.Load() != 1 {
				return fmt.Errorf("follow-up apply replayed mutation; count=%d", writes.Load())
			}
			return nil
		}},
		{Config: targetConfig, PlanOnly: true, Check: func(*terraform.State) error {
			if writes.Load() != 1 {
				return fmt.Errorf("no-op plan replayed mutation; count=%d", writes.Load())
			}
			return nil
		}},
		{Config: authConfig, ConfigVariables: authVars, Check: resourceTesting.ComposeAggregateTestCheckFunc(resourceTesting.TestCheckResourceAttr("qbittorrent_webui_auth.admin", "username", "operator"), func(state *terraform.State) error {
			if authWrites.Load() != 1 {
				return fmt.Errorf("auth mutation count=%d, want 1", authWrites.Load())
			}
			return checkNoCredentialCanaryInState(map[string]string{"webui": canary})(state)
		}), ConfigPlanChecks: resourceTesting.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{noCredentialCanaryPlanCheck{canaries: []string{canary}}}}},
		{Config: authConfig, ConfigVariables: authVars, PlanOnly: true, Check: func(state *terraform.State) error {
			if authWrites.Load() != 1 {
				return fmt.Errorf("auth no-op plan replayed write, count=%d", authWrites.Load())
			}
			return checkNoSecret(state)
		}},
		{Config: authConfig, ConfigVariables: authVars, PreConfig: func() { currentUsername.Store("external"); _ = os.Setenv("QBITTORRENT_USERNAME", "external") }, PlanOnly: true, ExpectNonEmptyPlan: true, Check: func(state *terraform.State) error {
			if authWrites.Load() != 1 {
				return fmt.Errorf("drift refresh wrote credentials, count=%d", authWrites.Load())
			}
			return checkNoSecret(state)
		}},
		{Config: authConfig, ConfigVariables: authVars, PreConfig: func() { currentUsername.Store("external"); _ = os.Setenv("QBITTORRENT_USERNAME", "external") }, Check: func(state *terraform.State) error {
			if authWrites.Load() != 2 {
				return fmt.Errorf("username drift update count=%d, want 2", authWrites.Load())
			}
			if authPasswordWrites.Load() != 1 {
				return fmt.Errorf("write-only password replayed on username drift; sends=%d", authPasswordWrites.Load())
			}
			return checkNoSecret(state)
		}},
		{Config: authConfig, ConfigVariables: authVars, PlanOnly: true, Check: func(state *terraform.State) error {
			if authWrites.Load() != 2 {
				return fmt.Errorf("auth follow-up no-op wrote, count=%d", authWrites.Load())
			}
			return checkNoSecret(state)
		}},
		{ResourceName: "qbittorrent_webui_auth.admin", ImportState: true, ImportStateId: "application", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password_wo_version"}, Config: authConfig, ConfigVariables: authVars},
	}})
}
