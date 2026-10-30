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
)

func TestAccAdvancedLibtorrentSafePersistenceAndRestart(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	baseline, err := c.Preferences(context.Background())
	if err != nil {
		t.Fatal("read initial qBittorrent preferences failed")
	}
	if baseline.DiskIOType == nil {
		t.Fatal("pinned qBittorrent response omitted disk_io_type")
	}
	container := os.Getenv("QBT_ACC_CONTAINER")
	restartContainer := func() error {
		if container == "" {
			return fmt.Errorf("controlled restart requires the disposable acceptance container")
		}
		if _, err := exec.Command("docker", "restart", container).CombinedOutput(); err != nil {
			return fmt.Errorf("restart disposable qBittorrent failed (%T)", err)
		}
		return nil
	}
	waitForDiskIO := func(expected int) error {
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			prefs, err := c.Preferences(context.Background())
			if err == nil && prefs.DiskIOType != nil && *prefs.DiskIOType == expected {
				return nil
			}
			time.Sleep(time.Second)
		}
		return fmt.Errorf("restart did not restore disk_io_type=%d", expected)
	}
	sameInt := func(a, b *int) bool {
		return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
	}
	sameInt64 := func(a, b *int64) bool {
		return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
	}
	waitForOriginal := func() error {
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			prefs, err := c.Preferences(context.Background())
			if err == nil && sameInt(prefs.DiskIOType, baseline.DiskIOType) &&
				sameInt64(prefs.DiskQueueSize, baseline.DiskQueueSize) &&
				sameInt(prefs.SocketSendBufferSize, baseline.SocketSendBufferSize) &&
				sameInt(prefs.SocketReceiveBufferSize, baseline.SocketReceiveBufferSize) &&
				sameInt(prefs.SendBufferWatermark, baseline.SendBufferWatermark) &&
				sameInt(prefs.SendBufferLowWatermark, baseline.SendBufferLowWatermark) &&
				sameInt(prefs.SendBufferWatermarkFactor, baseline.SendBufferWatermarkFactor) &&
				sameInt(prefs.UPnPLeaseDuration, baseline.UPnPLeaseDuration) &&
				sameInt(prefs.AnnouncePort, baseline.AnnouncePort) &&
				sameInt(prefs.PeerTurnover, baseline.PeerTurnover) &&
				sameInt(prefs.PeerTurnoverCutoff, baseline.PeerTurnoverCutoff) &&
				sameInt(prefs.PeerTurnoverInterval, baseline.PeerTurnoverInterval) {
				return nil
			}
			time.Sleep(time.Second)
		}
		return fmt.Errorf("server did not restore all original libtorrent values")
	}
	// Register cleanup before Terraform can mutate anything; acceptance teardown
	// may be skipped after a failed provider read or destroy.
	t.Cleanup(func() {
		if err := c.SetPreferencesPatch(context.Background(), client.PreferencesPatch{
			DiskIOType: baseline.DiskIOType, DiskQueueSize: baseline.DiskQueueSize,
			SocketSendBufferSize: baseline.SocketSendBufferSize, SocketReceiveBufferSize: baseline.SocketReceiveBufferSize,
			SendBufferWatermark: baseline.SendBufferWatermark, SendBufferLowWatermark: baseline.SendBufferLowWatermark,
			SendBufferWatermarkFactor: baseline.SendBufferWatermarkFactor, UPnPLeaseDuration: baseline.UPnPLeaseDuration,
			AnnouncePort: baseline.AnnouncePort, PeerTurnover: baseline.PeerTurnover,
			PeerTurnoverCutoff: baseline.PeerTurnoverCutoff, PeerTurnoverInterval: baseline.PeerTurnoverInterval,
		}); err != nil {
			t.Errorf("restore original libtorrent preferences failed (%T)", err)
			return
		}
		if err := restartContainer(); err != nil {
			t.Errorf("restart after restoring original libtorrent preferences failed: %v", err)
			return
		}
		if err := waitForOriginal(); err != nil {
			t.Errorf("verify restored libtorrent preferences failed: %v", err)
		}
	})
	config := accProvider() + `
resource "qbittorrent_preferences" "libtorrent" {
  # Explicit wire bytes, even though the UI displays KiB.
  disk_queue_size = 104857600
  socket_send_buffer_size = 65536
  socket_receive_buffer_size = 65536
  send_buffer_watermark = 512
  send_buffer_low_watermark = 128
  send_buffer_watermark_factor = 150
  upnp_lease_duration = 0
  announce_port = 0
  peer_turnover = 4
  peer_turnover_cutoff = 90
  peer_turnover_interval = 300
  # Common native enum value; qBittorrent documents disk_io_type as restart-required.
  disk_io_type = 3
}
data "qbittorrent_preferences" "libtorrent" {
  depends_on = [qbittorrent_preferences.libtorrent]
}
`
	restart := func() {
		t.Helper()
		if err := restartContainer(); err != nil {
			t.Fatal(err)
		}
		if err := waitForDiskIO(3); err != nil {
			t.Fatal(err)
		}
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			prefs, err := c.Preferences(context.Background())
			if err != nil {
				return err
			}
			if prefs.DiskQueueSize == nil || *prefs.DiskQueueSize != 104857600 {
				return fmt.Errorf("destroy unexpectedly reset disk_queue_size")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("qbittorrent_preferences.libtorrent", "disk_queue_size", "104857600"),
				resource.TestCheckResourceAttr("qbittorrent_preferences.libtorrent", "socket_send_buffer_size", "65536"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.libtorrent", "send_buffer_watermark", "512"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.libtorrent", "peer_turnover", "4"),
			)},
			{Config: config, PreConfig: restart, PlanOnly: true},
		},
	})
}
