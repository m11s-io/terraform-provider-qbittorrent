package provider

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

// This fixture has no announce URL and is stopped. DHT/PeX/LSD start disabled.
// It never fetches any content.
func TestAccCategoryDeletionPreservesTorrentAndData(t *testing.T) {
	c := accClient(t)
	ctx := context.Background()
	name := "provider-preservation"
	if err := c.CreateCategory(ctx, client.Category{Name: name, SavePath: "/downloads/keep"}); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("docker", "exec", os.Getenv("QBT_ACC_CONTAINER"), "sh", "-c", "mkdir -p /downloads/keep && printf hello > /downloads/keep/sample.txt")
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	piece := sha1.Sum([]byte("hello"))
	info := append([]byte("d6:lengthi5e4:name10:sample.txt12:piece lengthi16384e6:pieces20:"), piece[:]...)
	info = append(info, 'e')
	hash := sha1.Sum(info)
	hashString := hex.EncodeToString(hash[:])
	torrent := append([]byte("d4:info"), info...)
	torrent = append(torrent, 'e')
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("torrents", "sample.torrent")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(torrent)
	for k, v := range map[string]string{"category": name, "savepath": "/downloads/keep", "stopped": "true", "autoTMM": "false", "skip_checking": "false"} {
		writer.WriteField(k, v)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", os.Getenv("QBT_ACC_URL")+"/api/v2/torrents/add", &body)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("fixture upload status %d", response.StatusCode)
	}
	getTorrent := func() map[string]any {
		req, _ := http.NewRequest("GET", os.Getenv("QBT_ACC_URL")+"/api/v2/torrents/info?hashes="+url.QueryEscape(hashString), nil)
		req.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var torrents []map[string]any
		if err = json.NewDecoder(res.Body).Decode(&torrents); err != nil {
			t.Fatal(err)
		}
		if len(torrents) == 0 {
			return nil
		}
		return torrents[0]
	}
	var found map[string]any
	for i := 0; i < 50; i++ {
		found = getTorrent()
		if found != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if found == nil || found["category"] != name {
		t.Fatalf("fixture missing/category incorrect: %v", found)
	}
	s := testState(t, categorySchema(), map[string]attr.Value{"id": types.StringValue(name), "name": types.StringValue(name), "save_path": types.StringValue("/downloads/keep")})
	r := &categoryResource{client: c}
	var out resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: s}, &out)
	if out.Diagnostics.HasError() {
		t.Fatal(out.Diagnostics)
	}
	found = getTorrent()
	if found == nil || found["category"] != "" {
		t.Fatalf("torrent removed or still assigned: %v", found)
	}
	content, err := exec.Command("docker", "exec", os.Getenv("QBT_ACC_CONTAINER"), "cat", "/downloads/keep/sample.txt").Output()
	if err != nil || string(content) != "hello" {
		t.Fatal("torrent data lost")
	}
	if cats, err := c.Categories(context.Background()); err != nil {
		t.Fatal(err)
	} else if _, exists := cats[name]; exists {
		t.Fatal("category still exists")
	}
	fmt.Println("Offline stopped torrent and its data preserved after category deletion")
}
