package client

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestWatchFolderWireUnionPreservesIntegersAndCustomPaths(t *testing.T) {
	got, err := EncodeWatchFolders(map[string]WatchFolder{
		"/watch":         {SaveMode: WatchFolderModeWatchFolder},
		"/watch-default": {SaveMode: WatchFolderModeDefault},
		"/watch-custom":  {SaveMode: WatchFolderModeCustom, SavePath: "/downloads/custom"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"/watch":0,"/watch-custom":"/downloads/custom","/watch-default":1}` {
		t.Fatalf("wire union = %s", b)
	}
	decoded, err := DecodeWatchFolders(json.RawMessage(b))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]WatchFolder{
		"/watch":         {SaveMode: WatchFolderModeWatchFolder},
		"/watch-default": {SaveMode: WatchFolderModeDefault},
		"/watch-custom":  {SaveMode: WatchFolderModeCustom, SavePath: "/downloads/custom"},
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded folders = %#v, want %#v", decoded, want)
	}
}

func TestWatchFolderCustomPathEqualToWatchedPathNormalizes(t *testing.T) {
	got, err := EncodeWatchFolders(map[string]WatchFolder{
		"/watch": {SaveMode: WatchFolderModeCustom, SavePath: "/watch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["/watch"] != 0 {
		t.Fatalf("equal custom path must use native watched-folder mode, got %#v", got["/watch"])
	}
}

func TestDecodeWatchFoldersRejectsInvalidServerValues(t *testing.T) {
	for _, wire := range []string{
		`{"/watch":true}`,
		`{"/watch":2}`,
		`{"/watch":null}`,
		`{"/watch":[]}`,
	} {
		if _, err := DecodeWatchFolders(json.RawMessage(wire)); err == nil {
			t.Errorf("DecodeWatchFolders(%s) accepted invalid union value", wire)
		}
	}
}

func TestDecodeWatchFoldersOmittedAndEmptyAreDistinct(t *testing.T) {
	if _, err := DecodeWatchFolders(nil); err == nil {
		t.Fatal("omitted scan_dirs must remain distinguishable from an explicit empty object")
	}
	got, err := DecodeWatchFolders(json.RawMessage(`{}`))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("explicit empty scan_dirs = %#v, %v", got, err)
	}
}

func TestWatchFoldersUsePreferencesEndpoints(t *testing.T) {
	var writes int
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		switch r.URL.Path {
		case "/proxy/api/v2/app/preferences":
			_, _ = w.Write([]byte(`{"scan_dirs":{"/watch":0,"/default":1,"/custom":"/downloads"}}`))
		case "/proxy/api/v2/app/setPreferences":
			writes++
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
				t.Fatal(err)
			}
			if string(got["scan_dirs"]) != `{"/watch":0,"/watch-custom":"/downloads/custom"}` {
				t.Errorf("scan_dirs patch = %s", got["scan_dirs"])
			}
			if len(got) != 1 {
				t.Errorf("unexpected extra preference keys: %v", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}, false)

	got, err := c.WatchFolders(context.Background())
	if err != nil || len(got) != 3 || got["/default"].SaveMode != WatchFolderModeDefault {
		t.Fatalf("read watch folders = %#v, %v", got, err)
	}
	if err := c.SetWatchFolders(context.Background(), map[string]WatchFolder{
		"/watch":        {SaveMode: WatchFolderModeWatchFolder},
		"/watch-custom": {SaveMode: WatchFolderModeCustom, SavePath: "/downloads/custom"},
	}); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("writes = %d", writes)
	}
}
