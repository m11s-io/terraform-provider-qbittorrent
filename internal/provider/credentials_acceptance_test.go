package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCredentialsWriteOnlyLifecycle(t *testing.T) {
	setupAccEnv(t, false)
	initial := map[string]string{
		"smtp":   randomCredentialCanary(t),
		"proxy":  randomCredentialCanary(t),
		"dyndns": randomCredentialCanary(t),
	}
	rotated := map[string]string{
		"smtp":   randomCredentialCanary(t),
		"proxy":  randomCredentialCanary(t),
		"dyndns": randomCredentialCanary(t),
	}
	unchangedVersion := map[string]string{
		"smtp":   randomCredentialCanary(t),
		"proxy":  randomCredentialCanary(t),
		"dyndns": randomCredentialCanary(t),
	}
	rotatedExpected := map[string]string{"smtp": rotated["smtp"], "proxy": initial["proxy"], "dyndns": initial["dyndns"]}
	oldEnv := map[string]string{}
	defer func() {
		for _, name := range []string{"smtp", "proxy", "dyndns"} {
			key := "TF_VAR_" + name + "_canary"
			if old, exists := oldEnv[key]; exists {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()
	for _, name := range []string{"smtp", "proxy", "dyndns"} {
		key := "TF_VAR_" + name + "_canary"
		if old, exists := os.LookupEnv(key); exists {
			oldEnv[key] = old
		}
	}
	setCredentialCanaries(initial)
	config := accProvider() + `
variable "smtp_canary" {
  type = string
  sensitive = true
  ephemeral = true
}
variable "proxy_canary" {
  type = string
  sensitive = true
  ephemeral = true
}
variable "dyndns_canary" {
  type = string
  sensitive = true
  ephemeral = true
}
resource "qbittorrent_credentials" "application" {
  smtp_password_wo = var.smtp_canary
  smtp_password_wo_version = 1
  proxy_password_wo = var.proxy_canary
  proxy_password_wo_version = 1
  dyndns_password_wo = var.dyndns_canary
  dyndns_password_wo_version = 1
  depends_on = [qbittorrent_preferences.safe]
}
resource "qbittorrent_preferences" "safe" {
  mail_notification_enabled = false
  mail_notification_auth_enabled = false
  proxy_type = "None"
  proxy_auth_enabled = false
  proxy_bittorrent = false
  proxy_peer_connections = false
  proxy_rss = false
  proxy_misc = false
  dyndns_enabled = false
}
`
	rotatedConfig := strings.Replace(config, "smtp_password_wo_version = 1", "smtp_password_wo_version = 2", 1)
	relinquishedConfig := strings.Replace(rotatedConfig,
		"smtp_password_wo = var.smtp_canary\n  smtp_password_wo_version = 2",
		"smtp_password_wo = null\n  smtp_password_wo_version = null", 1)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(_ *terraform.State) error {
			return checkCredentialValues(rotatedExpected)
		},
		Steps: []resource.TestStep{
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{noCredentialCanaryPlanCheck{canaries: []string{initial["smtp"], initial["proxy"], initial["dyndns"]}}}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "id", "application"),
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "smtp_password_wo_version", "1"),
					resource.TestCheckNoResourceAttr("qbittorrent_credentials.application", "smtp_password_wo"),
					resource.TestCheckNoResourceAttr("qbittorrent_credentials.application", "proxy_password_wo"),
					resource.TestCheckNoResourceAttr("qbittorrent_credentials.application", "dyndns_password_wo"),
					checkNoCredentialCanaryInState(initial),
					checkCredentialCanaries(initial),
				),
			},
			{
				Config:           rotatedConfig,
				PreConfig:        func() { setCredentialCanaries(rotated) },
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{noCredentialCanaryPlanCheck{canaries: []string{rotated["smtp"], rotated["proxy"], rotated["dyndns"]}}}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "smtp_password_wo_version", "2"),
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "proxy_password_wo_version", "1"),
					checkNoCredentialCanaryInState(rotated),
					checkCredentialCanaries(rotatedExpected),
				),
			},
			{
				Config:    rotatedConfig,
				PreConfig: func() { setCredentialCanaries(unchangedVersion) },
				PlanOnly:  true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "smtp_password_wo_version", "2"),
					checkNoCredentialCanaryInState(initial, rotated, unchangedVersion),
					checkCredentialCanaries(rotatedExpected),
				),
			},
			{
				Config:    relinquishedConfig,
				PreConfig: func() { setCredentialCanaries(unchangedVersion) },
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{noCredentialCanaryPlanCheck{canaries: []string{
					unchangedVersion["proxy"], unchangedVersion["dyndns"],
				}}}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("qbittorrent_credentials.application", "smtp_password_wo_version"),
					resource.TestCheckResourceAttr("qbittorrent_credentials.application", "proxy_password_wo_version", "1"),
					checkNoCredentialCanaryInState(initial, rotated, unchangedVersion),
					checkCredentialCanaries(rotatedExpected),
				),
			},
			{
				ResourceName:      "qbittorrent_credentials.application",
				ImportState:       true,
				ImportStateId:     "application",
				ImportStateVerify: false,
			},
		},
	})
}

type noCredentialCanaryPlanCheck struct{ canaries []string }

func (c noCredentialCanaryPlanCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	b, err := json.Marshal(req.Plan)
	if err != nil {
		resp.Error = fmt.Errorf("cannot inspect plan JSON")
		return
	}
	for _, canary := range c.canaries {
		if strings.Contains(string(b), canary) {
			resp.Error = fmt.Errorf("write-only password was persisted in Terraform plan JSON")
			return
		}
	}
}

func randomCredentialCanary(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal("cannot create credential fixture")
	}
	return "tf-wo-" + hex.EncodeToString(buf)
}

func setCredentialCanaries(values map[string]string) {
	_ = os.Setenv("TF_VAR_smtp_canary", values["smtp"])
	_ = os.Setenv("TF_VAR_proxy_canary", values["proxy"])
	_ = os.Setenv("TF_VAR_dyndns_canary", values["dyndns"])
}

func checkNoCredentialCanaryInState(values ...map[string]string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		for _, canaries := range values {
			for _, canary := range canaries {
				for _, resourceState := range state.RootModule().Resources {
					for _, value := range resourceState.Primary.Attributes {
						if strings.Contains(value, canary) {
							return fmt.Errorf("write-only password was persisted in Terraform state")
						}
					}
				}
			}
		}
		return nil
	}
}

func checkCredentialCanaries(expected map[string]string) resource.TestCheckFunc {
	return func(_ *terraform.State) error { return checkCredentialValues(expected) }
}

func checkCredentialValues(expected map[string]string) error {
	url := strings.TrimRight(os.Getenv("QBT_ACC_URL"), "/") + "/api/v2/app/preferences"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("cannot construct fixture request")
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot read isolated credential fixture")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("credential fixture read returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("cannot read isolated credential fixture")
	}
	var prefs map[string]any
	if err := json.Unmarshal(body, &prefs); err != nil {
		return fmt.Errorf("invalid credential fixture response")
	}
	want := map[string]string{
		"mail_notification_password": expected["smtp"],
		"proxy_password":             expected["proxy"],
		"dyndns_password":            expected["dyndns"],
	}
	for key, value := range want {
		if prefs[key] != value {
			return fmt.Errorf("remote %s did not match expected lifecycle value", key)
		}
	}
	return nil
}
