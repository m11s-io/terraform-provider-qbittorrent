package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDownloadsPreferences(t *testing.T) {
	setupAccEnv(t, false)
	config := accProvider() + `
resource "qbittorrent_preferences" "downloads" {
  torrent_content_layout = "Subfolder"
  add_to_top_of_queue = false
  add_stopped_enabled = true
  torrent_stop_condition = "FilesChecked"
  merge_trackers = false
  auto_delete_mode = 0
  preallocate_all = false
  incomplete_files_ext = false
  use_unwanted_folder = false
  torrent_changed_tmm_enabled = false
  save_path_changed_tmm_enabled = false
  category_changed_tmm_enabled = false
  use_category_paths_in_manual_mode = true
  torrent_files_backup_enabled = false
  torrent_files_backup_dir = "/config/provider-torrent-files"
  torrent_files_finished_backup_dir_enabled = false
  torrent_files_finished_backup_dir = "/config/provider-finished-torrent-files"
  remove_torrent_file_backup = false
  excluded_file_names_enabled = true
  excluded_file_names = "*.provider-test-tmp\n*.provider-test-part\n"
  mail_notification_enabled = false
  mail_notification_sender = "provider-test@example.invalid"
  mail_notification_email = "one@example.invalid,two@example.invalid"
  mail_notification_smtp = "smtp.example.invalid:587"
  mail_notification_encryption_type = "STARTTLS"
  mail_notification_auth_enabled = false
  mail_notification_username = "provider-test"
  autorun_on_torrent_added_enabled = false
  autorun_on_torrent_added_program = "/usr/bin/printf added"
  autorun_enabled = false
  autorun_program = "/usr/bin/printf finished"
}
data "qbittorrent_preferences" "downloads" {
  depends_on = [qbittorrent_preferences.downloads]
}
`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("qbittorrent_preferences.downloads", "auto_delete_mode", "0"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.downloads", "add_to_top_of_queue", "false"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.downloads", "torrent_files_backup_dir", "/config/provider-torrent-files"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.downloads", "excluded_file_names", "*.provider-test-tmp\n*.provider-test-part\n"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.downloads", "autorun_enabled", "false"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.downloads", "mail_notification_enabled", "false"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.downloads", "mail_notification_encryption_type", "STARTTLS"),
		)},
		{Config: config, PlanOnly: true},
	}})
}
