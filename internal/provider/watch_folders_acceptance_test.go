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
	"reflect"
	"regexp"
	"sort"
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
		// Match the runner's PUID/PGID so Linux can remove the bind-mounted
		// directory after the container stops, including its mode-0700 entries.
		user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
		cmd := exec.Command("docker", append([]string{"exec", "--user", user, container}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("disposable watched-folder setup failed (%s)", strings.TrimSpace(string(output)))
		}
		return nil
	}
	if err := dockerExec("mkdir", "-m", "700", watchPath); err != nil {
		t.Fatal(err)
	}
	torrentBaseline, err := accTorrentHashes(t)
	if err != nil {
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

	checkWatch := func(expected client.WatchFolderMode, expectMarker bool) resource.TestCheckFunc {
		return func(_ *terraform.State) error {
			folders, err := c.WatchFolders(context.Background())
			if err != nil {
				return err
			}
			if len(folders) != 1 || folders[watchPath].SaveMode != expected {
				return fmt.Errorf("server watched-folder map = %#v, want %q at disposable path", folders, expected)
			}
			return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, expectMarker, torrentBaseline)
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
		return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, true, torrentBaseline)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(_ *terraform.State) error {
			folders, err := c.WatchFolders(context.Background())
			if err != nil {
				return err
			}
			if len(folders) != 1 || folders[watchPath].SaveMode != client.WatchFolderModeWatchFolder {
				return fmt.Errorf("destroy changed non-empty server watches: %#v", folders)
			}
			return checkNoTorrentOrUnexpectedFiles(t, watchPath, marker, true, torrentBaseline)
		},
		Steps: []resource.TestStep{
			{Config: config, Check: checkWatch(client.WatchFolderModeDefault, false)},
			{Config: config, PreConfig: func() {
				if err := c.SetWatchFolders(context.Background(), map[string]client.WatchFolder{}); err != nil {
					t.Fatal(err)
				}
			}, Check: checkWatch(client.WatchFolderModeDefault, false)},
			{ResourceName: "qbittorrent_watch_folders.application", ImportState: true, ImportStateId: "application", ImportStateVerify: true},
			{Config: withoutOwnership, PreConfig: func() {
				if err := dockerExec("touch", marker); err != nil {
					t.Fatal(err)
				}
				if err := c.SetWatchFolders(context.Background(), map[string]client.WatchFolder{watchPath: {SaveMode: client.WatchFolderModeWatchFolder}}); err != nil {
					t.Fatal(err)
				}
			}, Check: checkWatch(client.WatchFolderModeWatchFolder, true)},
			{Config: config, ExpectError: regexp.MustCompile("Cannot switch watched folder to default in one apply")},
			{Config: clear, Check: checkCleared},
			{Config: accProvider() + fmt.Sprintf(`resource "qbittorrent_watch_folders" "application" {
  folders = { %q = { save_mode = "watch_folder" } }
}
`, watchPath), Check: checkWatch(client.WatchFolderModeWatchFolder, true)},
		},
	})
}

func accTorrentHashes(t *testing.T) ([]string, error) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(os.Getenv("QBT_ACC_URL"), "/")+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect disposable torrent list")
	}
	request.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect disposable torrent list")
	}
	defer response.Body.Close()
	var torrents []struct {
		Hash string `json:"hash"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&torrents) != nil {
		return nil, fmt.Errorf("cannot inspect disposable torrent list")
	}
	hashes := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		hashes = append(hashes, torrent.Hash)
	}
	sort.Strings(hashes)
	return hashes, nil
}

func checkNoTorrentOrUnexpectedFiles(t *testing.T, watchPath, marker string, expectMarker bool, baseline []string) error {
	t.Helper()
	hashes, err := accTorrentHashes(t)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(hashes, baseline) {
		return fmt.Errorf("watched-folder acceptance changed the disposable server torrent list")
	}
	container := os.Getenv("QBT_ACC_CONTAINER")
	cmd := exec.Command("docker", "exec", container, "sh", "-c", "test -d \"$1\" && if test \"$2\" = yes; then test -f \"$3\"; else test ! -e \"$3\"; fi && test \"$(find \"$1\" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')\" -eq \"$4\"", "watch-folder-check", watchPath, map[bool]string{true: "yes", false: "no"}[expectMarker], marker, map[bool]string{true: "1", false: "0"}[expectMarker])
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("watched-folder acceptance changed disposable directory contents (%s)", strings.TrimSpace(string(output)))
	}
	return nil
}
