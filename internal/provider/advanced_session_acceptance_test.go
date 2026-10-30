package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

func TestAccAdvancedSessionPreferences(t *testing.T) {
	setupAccEnv(t, false)
	config := accProvider() + `resource "qbittorrent_preferences" "advanced" {
  torrent_file_size_limit = 104857600
  shutdown_timeout = -1
  confirm_torrent_recheck = false
  recheck_completed_torrents = false
  enable_embedded_tracker = false
  ignore_ssl_errors = false
}
data "qbittorrent_preferences" "advanced" {
  depends_on = [qbittorrent_preferences.advanced]
}`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("qbittorrent_preferences.advanced", "torrent_file_size_limit", "104857600"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.advanced", "shutdown_timeout", "-1"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.advanced", "torrent_file_size_limit", "104857600"),
		)},
		{Config: config, PlanOnly: true},
	}})
}

func TestAccAdvancedSessionRestartPersistence(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	initial, err := c.Preferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if initial.ResumeDataStorageType == nil {
		t.Fatal("server did not return the initial resume-data storage backend")
	}
	originalBackend := string(*initial.ResumeDataStorageType)
	container := os.Getenv("QBT_ACC_CONTAINER")
	if container == "" {
		t.Fatal("use tests/acceptance/run.py to create an isolated disposable server")
	}
	config := accProvider() + `resource "qbittorrent_preferences" "advanced" {
  resume_data_storage_type = "SQLite"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			backend := openapi.PreferencesPatchResumeDataStorageType(originalBackend)
			if err := c.SetPreferencesPatch(context.Background(), client.PreferencesPatch{ResumeDataStorageType: &backend}); err != nil {
				return fmt.Errorf("restore original resume-data storage backend: %w", err)
			}
			if originalBackend != "SQLite" {
				if err := restartDisposableQBT(c, container, originalBackend); err != nil {
					return fmt.Errorf("restart disposable server after restoring original resume-data backend: %w", err)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttr("qbittorrent_preferences.advanced", "resume_data_storage_type", "SQLite")},
			{Config: config, PreConfig: func() {
				if err := restartDisposableQBT(c, container, "SQLite"); err != nil {
					t.Fatal(err)
				}
			}, PlanOnly: true},
		},
	})
}

func restartDisposableQBT(c *client.Client, container, expectedBackend string) error {
	if err := exec.Command("docker", "restart", container).Run(); err != nil {
		return fmt.Errorf("restart disposable qBittorrent container: %w", err)
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		preferences, err := c.Preferences(ctx)
		cancel()
		if err == nil && preferences.ResumeDataStorageType != nil && string(*preferences.ResumeDataStorageType) == expectedBackend {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("qBittorrent did not return resume-data backend %q after disposable restart", expectedBackend)
}
