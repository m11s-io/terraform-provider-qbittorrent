package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
	"os"
	"regexp"
	"strings"
	"testing"
)

var factories = map[string]func() (tfprotov6.ProviderServer, error){"qbittorrent": providerserver.NewProtocol6WithError(New("test")())}

func accClient(t *testing.T) *client.Client {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance is opt-in")
	}
	if os.Getenv("QBT_ACC_CONTAINER") == "" {
		t.Fatal("use tests/acceptance/run.py to create an isolated server")
	}
	c, err := client.New(client.Config{URL: os.Getenv("QBT_ACC_URL"), APIKey: os.Getenv("QBT_ACC_API_KEY")})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func accProvider() string {
	return `terraform {
 required_providers {
  qbittorrent = { source = "registry.terraform.io/m11s-io/qbittorrent" }
 }
}
provider "qbittorrent" {}
`
}
func setupAccEnv(t *testing.T, session bool) {
	t.Helper()
	accClient(t)
	t.Setenv("TF_ACC_PROVIDER_NAMESPACE", "m11s-io")
	t.Setenv("TF_ACC_PROVIDER_HOST", "registry.terraform.io")
	t.Setenv("QBITTORRENT_URL", os.Getenv("QBT_ACC_URL"))
	if session {
		t.Setenv("QBITTORRENT_API_KEY", "")
		t.Setenv("QBITTORRENT_USERNAME", os.Getenv("QBT_ACC_USERNAME"))
		t.Setenv("QBITTORRENT_PASSWORD", os.Getenv("QBT_ACC_PASSWORD"))
	} else {
		t.Setenv("QBITTORRENT_API_KEY", os.Getenv("QBT_ACC_API_KEY"))
		t.Setenv("QBITTORRENT_USERNAME", "")
		t.Setenv("QBITTORRENT_PASSWORD", "")
	}
}
func TestAccPreferences(t *testing.T) {
	for _, session := range []bool{false, true} {
		t.Run(fmt.Sprintf("session=%v", session), func(t *testing.T) {
			setupAccEnv(t, session)
			c := accClient(t)
			config := accProvider() + `resource "qbittorrent_preferences" "test" {
   dht = false
   dl_limit = 2048
  }
  data "qbittorrent_preferences" "current" { depends_on = [qbittorrent_preferences.test] }
  `
			updated := strings.Replace(config, "2048", "4096", 1)
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories,
				CheckDestroy: func(*terraform.State) error {
					p, err := c.Preferences(context.Background())
					if err != nil {
						return err
					}
					if p.DownloadLimit == nil || *p.DownloadLimit != 4096 {
						return fmt.Errorf("destroy reset preferences")
					}
					return nil
				},
				Steps: []resource.TestStep{
					{Config: config, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("qbittorrent_preferences.test", "id", "application"), resource.TestCheckResourceAttr("data.qbittorrent_preferences.current", "dl_limit", "2048"))},
					{Config: config, PlanOnly: true},
					{ResourceName: "qbittorrent_preferences.test", ImportState: true, ImportStateId: "application", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"dht", "dl_limit"}},
					{Config: updated},
					{Config: updated, PreConfig: func() {
						z := int64(8192)
						if err := c.SetPreferences(context.Background(), client.Preferences{DownloadLimit: &z}); err != nil {
							t.Fatal(err)
						}
					}, PlanOnly: true, ExpectNonEmptyPlan: true},
					{Config: updated},
					{Config: updated, PlanOnly: true},
				},
			})
		})
	}
}
func TestAccSpeedPreferences(t *testing.T) {
	setupAccEnv(t, false)
	config := accProvider() + `
resource "qbittorrent_preferences" "speed" {
  dl_limit = 2097152
  up_limit = 30720
  alt_dl_limit = 10240
  alt_up_limit = 10240
  scheduler_enabled = false
  schedule_from_hour = 8
  schedule_from_min = 0
  schedule_to_hour = 20
  schedule_to_min = 0
  scheduler_days = 1
  limit_utp_rate = true
  limit_tcp_overhead = false
  limit_lan_peers = true
}
data "qbittorrent_preferences" "speed" {
  depends_on = [qbittorrent_preferences.speed]
}
`
	updated := strings.Replace(config, "scheduler_enabled = false", "scheduler_enabled = true", 1)
	updated = strings.Replace(updated, "scheduler_days = 1", "scheduler_days = 9", 1)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("qbittorrent_preferences.speed", "schedule_from_min", "0"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.speed", "scheduler_days", "1"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.speed", "limit_tcp_overhead", "false"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.speed", "limit_lan_peers", "true"),
		)},
		{Config: config, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("qbittorrent_preferences.speed", "scheduler_enabled", "true"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.speed", "scheduler_days", "9"),
		)},
		{Config: updated, PlanOnly: true},
	}})
}

func TestAccCategory(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	config := accProvider() + `
 resource "qbittorrent_category" "test" {
 name = "provider-acceptance"
 save_path = "/downloads/acceptance"
 }`
	updated := strings.Replace(config, "/downloads/acceptance", "/downloads/changed", 1)
	renamed := strings.Replace(updated, `"provider-acceptance"`, `"provider-acceptance-renamed"`, 1)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, CheckDestroy: func(*terraform.State) error {
		cats, err := c.Categories(context.Background())
		if err != nil {
			return err
		}
		if _, ok := cats["provider-acceptance"]; ok {
			return fmt.Errorf("category retained")
		}
		if _, ok := cats["provider-acceptance-renamed"]; ok {
			return fmt.Errorf("renamed category retained")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttr("qbittorrent_category.test", "id", "provider-acceptance")},
		{Config: config, PlanOnly: true},
		{ResourceName: "qbittorrent_category.test", ImportState: true, ImportStateVerify: true},
		{Config: updated},
		{Config: renamed, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("qbittorrent_category.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
		{Config: renamed},
		{Config: renamed, PreConfig: func() {
			if err := c.DeleteCategory(context.Background(), "provider-acceptance-renamed"); err != nil {
				t.Fatal(err)
			}
		}, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: renamed},
	}})
}
func TestAccInvalidCredentials(t *testing.T) {
	setupAccEnv(t, false)
	t.Setenv("QBITTORRENT_API_KEY", "qbt_0000000000000000000000000000")
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{{Config: accProvider() + `data "qbittorrent_preferences" "current" {}`, ExpectError: regexp.MustCompile("HTTP 403")}}})
}

func TestAccPreferencesRelinquishOwnership(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	config := accProvider() + `resource "qbittorrent_preferences" "test" { alt_up_limit = 0 }`
	unconfigured := accProvider() + `resource "qbittorrent_preferences" "test" {}`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttr("qbittorrent_preferences.test", "alt_up_limit", "0")},
		{Config: unconfigured},
		{Config: unconfigured, PreConfig: func() {
			n := int64(2048)
			if err := c.SetPreferences(context.Background(), client.Preferences{AltUploadLimit: &n}); err != nil {
				t.Fatal(err)
			}
		}, PlanOnly: true},
	}})
}

func TestAccNestedCategory(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	config := accProvider() + `resource "qbittorrent_category" "parent" {
 name = "provider-parent"
 save_path = ""
 }
 resource "qbittorrent_category" "child" {
 name = "${qbittorrent_category.parent.name}/child"
 save_path = ""
 }`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, CheckDestroy: func(*terraform.State) error {
		cats, err := c.Categories(context.Background())
		if err != nil {
			return err
		}
		for name := range cats {
			if strings.HasPrefix(name, "provider-parent") {
				return fmt.Errorf("nested category leaked: %s", name)
			}
		}
		return nil
	}, Steps: []resource.TestStep{{Config: config}, {Config: config, PlanOnly: true}}})
	missing := accProvider() + `resource "qbittorrent_category" "child" {
 name = "provider-missing/child"
 save_path = ""
 }`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{{Config: missing, ExpectError: regexp.MustCompile("Missing ancestor category")}}})
	cats, err := c.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for name := range cats {
		if strings.HasPrefix(name, "provider-missing") {
			t.Fatal("missing ancestor was silently created")
		}
	}
}
