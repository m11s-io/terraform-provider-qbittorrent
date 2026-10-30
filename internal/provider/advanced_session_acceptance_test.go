package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		err := restoreAdvancedBackend(
			cleanupCtx,
			originalBackend,
			func(ctx context.Context) (string, error) {
				preferences, err := c.Preferences(ctx)
				if err != nil {
					return "", err
				}
				if preferences.ResumeDataStorageType == nil {
					return "", errors.New("server omitted resume-data storage backend")
				}
				return string(*preferences.ResumeDataStorageType), nil
			},
			func(ctx context.Context, value string) error {
				backend := openapi.PreferencesPatchResumeDataStorageType(value)
				return c.SetPreferencesPatch(ctx, client.PreferencesPatch{ResumeDataStorageType: &backend})
			},
			func(ctx context.Context) error { return restartDisposableContainer(ctx, container) },
		)
		if err != nil {
			t.Errorf("restore original resume-data backend after acceptance test: %v", err)
		}
	})
	config := accProvider() + `resource "qbittorrent_preferences" "advanced" {
  resume_data_storage_type = "SQLite"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttr("qbittorrent_preferences.advanced", "resume_data_storage_type", "SQLite")},
			{Config: config, PreConfig: func() {
				restartCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				if err := restartDisposableContainer(restartCtx, container); err != nil {
					t.Fatal(err)
				}
				if _, err := waitForAdvancedBackend(restartCtx, func(ctx context.Context) (string, error) {
					preferences, err := c.Preferences(ctx)
					if err != nil {
						return "", err
					}
					if preferences.ResumeDataStorageType == nil {
						return "", errors.New("server omitted resume-data storage backend")
					}
					return string(*preferences.ResumeDataStorageType), nil
				}, "SQLite"); err != nil {
					t.Fatal(err)
				}
			}, PlanOnly: true},
		},
	})
}

func restoreAdvancedBackend(parent context.Context, original string, read func(context.Context) (string, error), write func(context.Context, string) error, restart func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	if err := restart(ctx); err != nil {
		return fmt.Errorf("restart disposable qBittorrent container for cleanup: %w", err)
	}
	backend, err := waitForAdvancedBackend(ctx, read, "")
	if err != nil {
		return fmt.Errorf("read backend after disposable recovery restart: %w", err)
	}
	if backend != original {
		if err := write(ctx, original); err != nil {
			return fmt.Errorf("restore original resume-data backend: %w", err)
		}
		if err := restart(ctx); err != nil {
			return fmt.Errorf("restart disposable qBittorrent container after backend restoration: %w", err)
		}
		backend, err = waitForAdvancedBackend(ctx, read, original)
		if err != nil {
			return fmt.Errorf("verify restored resume-data backend: %w", err)
		}
	}
	if backend != original {
		return fmt.Errorf("resume-data backend readback %q; want original %q", backend, original)
	}
	return nil
}

func waitForAdvancedBackend(ctx context.Context, read func(context.Context) (string, error), expected string) (string, error) {
	var lastErr error
	for {
		backend, err := read(ctx)
		if err == nil && (expected == "" || backend == expected) {
			return backend, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("resume-data backend is %q; waiting for %q", backend, expected)
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("qBittorrent did not return expected resume-data backend before timeout: %w (last result: %v)", ctx.Err(), lastErr)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func restartDisposableContainer(ctx context.Context, container string) error {
	if err := exec.CommandContext(ctx, "docker", "restart", container).Run(); err != nil {
		return fmt.Errorf("restart disposable qBittorrent container: %w", err)
	}
	return nil
}
