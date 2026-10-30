package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func TestAccWatchFoldersSafeLifecycle(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	container := os.Getenv("QBT_ACC_CONTAINER")
	if container == "" || strings.ContainsAny(container, " \t\n/'\"") {
		t.Fatal("acceptance runner must provide a simple disposable container name")
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal("cannot create disposable watched-folder name")
	}
	watchPath := "/config/qbt-watch-" + hex.EncodeToString(suffix[:])
	marker := watchPath + "/preserve-me"
	dockerExec := func(args ...string) error {
		cmd := exec.Command("docker", append([]string{"exec", container}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("disposable watched-folder setup failed (%s)", strings.TrimSpace(string(output)))
		}
		return nil
	}
	if err := dockerExec("mkdir", "-m", "700", watchPath); err != nil {
		t.Fatal(err)
	}
	config := accProvider() + fmt.Sprintf(`
resource "qbittorrent_watch_folders" "application" {
  folders = {
    %q = { save_mode = "default" }
  }
}
`, watchPath)
	withoutOwnership := accProvider() + `resource "qbittorrent_watch_folders" "application" {}`
	clear := accProvider() + `resource "qbittorrent_watch_folders" "application" { folders = {} }`

	checkWatch := func(expected client.WatchFolderMode) resource.TestCheckFunc {
		return func(_ *terraform.State) error {
			folders, err := c.WatchFolders(context.Background())
			if err != nil {
				return err
			}
			if len(folders) != 1 || folders[watchPath].SaveMode != expected {
				return fmt.Errorf("server watched-folder map = %#v, want %q at disposable path", folders, expected)
			}
			return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, false)
		}
	}
	checkCleared := func(_ *terraform.State) error {
		folders, err := c.WatchFolders(context.Background())
		if err != nil {
			return err
		}
		if len(folders) != 0 {
			return fmt.Errorf("empty folders did not clear complete map: %#v", folders)
		}
		return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, true)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(_ *terraform.State) error {
			folders, err := c.WatchFolders(context.Background())
			if err != nil {
				return err
			}
			if len(folders) != 0 {
				return fmt.Errorf("destroy changed server watches: %#v", folders)
			}
			return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, true)
		},
		Steps: []resource.TestStep{
			{Config: config, Check: checkWatch(client.WatchFolderModeDefault)},
			{Config: config, PreConfig: func() {
				if err := c.SetWatchFolders(context.Background(), map[string]client.WatchFolder{watchPath: {SaveMode: client.WatchFolderModeWatchFolder}}); err != nil {
					t.Fatal(err)
				}
			}, Check: checkWatch(client.WatchFolderModeDefault)},
			{ResourceName: "qbittorrent_watch_folders.application", ImportState: true, ImportStateId: "application", ImportStateVerify: true},
			{Config: withoutOwnership, PreConfig: func() {
				if err := dockerExec("touch", marker); err != nil {
					t.Fatal(err)
				}
				if err := c.SetWatchFolders(context.Background(), map[string]client.WatchFolder{watchPath: {SaveMode: client.WatchFolderModeWatchFolder}}); err != nil {
					t.Fatal(err)
				}
			}, Check: checkWatch(client.WatchFolderModeWatchFolder)},
			{Config: clear, Check: checkCleared},
		},
	})
}

func checkNoTorrentOrUnexpectedFiles(t *testing.T, watchPath, marker string, expectMarker bool) error {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(os.Getenv("QBT_ACC_URL"), "/")+"/api/v2/torrents/info", nil)
	if err != nil {
		return fmt.Errorf("cannot verify disposable torrent list")
	}
	request.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("cannot verify disposable torrent list")
	}
	defer response.Body.Close()
	var torrents []json.RawMessage
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&torrents) != nil || len(torrents) != 0 {
		return fmt.Errorf("watched-folder acceptance must not add torrents")
	}
	container := os.Getenv("QBT_ACC_CONTAINER")
	cmd := exec.Command("docker", "exec", container, "sh", "-c", "test -d \"$1\" && if test \"$2\" = yes; then test -f \"$3\"; else test ! -e \"$3\"; fi && test \"$(find \"$1\" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')\" -eq \"$4\"", "watch-folder-check", watchPath, map[bool]string{true: "yes", false: "no"}[expectMarker], marker, map[bool]string{true: "1", false: "0"}[expectMarker])
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("watched-folder acceptance changed disposable directory contents (%s)", strings.TrimSpace(string(output)))
	}
	return nil
}
