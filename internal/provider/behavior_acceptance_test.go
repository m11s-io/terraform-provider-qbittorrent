package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccWebUIPreferences(t *testing.T) {
	for _, session := range []bool{false, true} {
		t.Run(fmt.Sprint(session), func(t *testing.T) {
			setupAccEnv(t, session)
			c := accClient(t)
			config := accProvider() + `
resource "qbittorrent_webui_preferences" "test" {
 date_format = "yyyy-MM-dd HH:mm:ss"
 color_scheme = "dark"
 display_density = "compact"
 use_alt_row_colors = true
 dblclick_download = "0"
 dblclick_complete = "0"
 dblclick_filter = "0"
 hide_zero_status_filters = true
 add_new_torrent_dialog_enabled = false
 full_url_tracker_column = true
 use_virtual_list = false
}
`
			updated := strings.ReplaceAll(strings.Replace(config, `color_scheme = "dark"`, `color_scheme = "auto"`, 1), `display_density = "compact"`, `display_density = "default"`)
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, CheckDestroy: func(*terraform.State) error {
				got, err := c.WebUIPreferences(context.Background())
				if err != nil {
					return err
				}
				if got["color_scheme"] != "auto" || got["use_virtual_list"] != false {
					return fmt.Errorf("destroy reset WebUI settings")
				}
				return nil
			}, Steps: []resource.TestStep{
				{Config: config, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("qbittorrent_webui_preferences.test", "color_scheme", "dark"), resource.TestCheckResourceAttr("qbittorrent_webui_preferences.test", "use_virtual_list", "false"))},
				{Config: config, PlanOnly: true},
				{ResourceName: "qbittorrent_webui_preferences.test", ImportState: true, ImportStateId: "application", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"date_format", "color_scheme", "display_density", "use_alt_row_colors", "dblclick_download", "dblclick_complete", "dblclick_filter", "hide_zero_status_filters", "add_new_torrent_dialog_enabled", "full_url_tracker_column", "use_virtual_list"}},
				{Config: updated, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("qbittorrent_webui_preferences.test", "color_scheme", "auto"), resource.TestCheckResourceAttr("qbittorrent_webui_preferences.test", "display_density", "default"))},
				{Config: updated, PlanOnly: true},
				{Config: updated, PreConfig: func() {
					if err := c.SetWebUIPreferences(context.Background(), map[string]any{"color_scheme": "light"}); err != nil {
						t.Fatal(err)
					}
				}, PlanOnly: true, ExpectNonEmptyPlan: true},
				{Config: updated},
			}})
		})
	}
}

func TestAccBehaviorPreferences(t *testing.T) {
	setupAccEnv(t, false)
	config := accProvider() + `
resource "qbittorrent_preferences" "behavior" {
 locale = "en"
 confirm_torrent_deletion = true
 status_bar_external_ip = false
 store_search_jobs = true
 store_search_job_results = false
 performance_warning = false
 file_log_enabled = false
 file_log_path = "/config/provider-logs"
 file_log_backup_enabled = true
 file_log_max_size = 65
 file_log_delete_old = true
 file_log_age = 1
 file_log_age_type = 0
}
data "qbittorrent_preferences" "behavior" {depends_on=[qbittorrent_preferences.behavior]}
`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("data.qbittorrent_preferences.behavior", "file_log_max_size", "65"), resource.TestCheckResourceAttr("data.qbittorrent_preferences.behavior", "file_log_age_type", "0"), resource.TestCheckResourceAttr("data.qbittorrent_preferences.behavior", "store_search_job_results", "false"))},
		{Config: config, PlanOnly: true},
	}})
}
