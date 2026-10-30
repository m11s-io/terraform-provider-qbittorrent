package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func watchFoldersSchema() rs.Schema {
	var out resource.SchemaResponse
	(&watchFoldersResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	return out.Schema
}

func watchFoldersTestState(t *testing.T, s rs.Schema, folders types.Map) tfsdk.State {
	t.Helper()
	attrTypes := s.Type().(types.ObjectType).AttrTypes
	value, diagnostics := types.ObjectValue(attrTypes, map[string]attr.Value{
		"id":      types.StringNull(),
		"folders": folders,
	})
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	state := tfsdk.State{Schema: s}
	state.Set(context.Background(), value)
	return state
}

func configuredWatchFolders(t *testing.T, entries map[string]client.WatchFolder) types.Map {
	t.Helper()
	value, err := watchFoldersValue(entries)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestWatchFoldersSchemaAndValidation(t *testing.T) {
	s := watchFoldersSchema()
	if _, ok := s.Attributes["folders"].(rs.MapNestedAttribute); !ok {
		t.Fatalf("folders must be a typed map of nested objects, got %T", s.Attributes["folders"])
	}
	for _, tc := range []struct {
		name string
		path map[string]client.WatchFolder
		bad  bool
	}{
		{name: "default", path: map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeDefault}}},
		{name: "watch folder", path: map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeWatchFolder}}},
		{name: "custom", path: map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeCustom, SavePath: "/downloads"}}},
		{name: "missing custom path", path: map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeCustom}}, bad: true},
		{name: "stray save path", path: map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeDefault, SavePath: "/downloads"}}, bad: true},
		{name: "unsupported mode", path: map[string]client.WatchFolder{"/watch": {SaveMode: "other"}}, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := client.ValidateWatchFolders(tc.path)
			if (err != nil) != tc.bad {
				t.Fatalf("ValidateWatchFolders error = %v, want error %v", err, tc.bad)
			}
		})
	}
}

func TestWatchFoldersApplyReplacementEmptyRelinquishmentAndImport(t *testing.T) {
	remote := map[string]any{"/existing": 1}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			writes++
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			values, _ := url.ParseQuery(r.Form.Encode())
			var patch map[string]any
			if err := json.Unmarshal([]byte(values.Get("json")), &patch); err != nil {
				t.Fatal(err)
			}
			if len(patch) != 1 {
				t.Fatalf("patch had unrelated keys: %#v", patch)
			}
			remote = patch["scan_dirs"].(map[string]any)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		b, _ := json.Marshal(map[string]any{"scan_dirs": remote})
		_, _ = w.Write(b)
	})
	r := &watchFoldersResource{client: c}
	s := watchFoldersSchema()
	configured := map[string]client.WatchFolder{
		"/watch":         {SaveMode: client.WatchFolderModeWatchFolder},
		"/watch-default": {SaveMode: client.WatchFolderModeDefault},
		"/watch-custom":  {SaveMode: client.WatchFolderModeCustom, SavePath: "/downloads"},
	}
	state := watchFoldersTestState(t, s, configuredWatchFolders(t, configured))
	var create resource.CreateResponse
	create.State = tfsdk.State{Schema: s}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	if writes != 1 || len(remote) != 3 || remote["/watch"] != float64(0) || remote["/watch-default"] != float64(1) || remote["/watch-custom"] != "/downloads" {
		t.Fatalf("replacement patch = %#v, writes=%d", remote, writes)
	}

	// A complete empty map is a write that clears server watches.
	emptyState := watchFoldersTestState(t, s, configuredWatchFolders(t, map[string]client.WatchFolder{}))
	var empty resource.UpdateResponse
	empty.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: emptyState.Raw}, State: create.State}, &empty)
	if empty.Diagnostics.HasError() || writes != 2 || len(remote) != 0 {
		t.Fatalf("empty replacement: remote=%#v writes=%d diag=%v", remote, writes, empty.Diagnostics)
	}

	// Removing the folders attribute relinquishes ownership without a write.
	nullState := watchFoldersTestState(t, s, types.MapNull(watchFolderObjectType))
	var relinquish resource.UpdateResponse
	relinquish.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: nullState.Raw}, State: empty.State}, &relinquish)
	if relinquish.Diagnostics.HasError() || writes != 2 {
		t.Fatalf("relinquish wrote or failed: writes=%d diag=%v", writes, relinquish.Diagnostics)
	}
	remote["/preserved"] = 1
	var imported resource.ImportStateResponse
	imported.State = watchFoldersTestState(t, s, types.MapNull(watchFolderObjectType))
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "application"}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	var importedFolders types.Map
	imported.State.GetAttribute(context.Background(), path.Root("folders"), &importedFolders)
	if importedFolders.IsNull() || len(importedFolders.Elements()) != 1 {
		t.Fatalf("import did not adopt the complete server map: %s", imported.State.Raw)
	}

	// Destroy never mutates the map.
	var deleted resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: imported.State}, &deleted)
	if deleted.Diagnostics.HasError() || writes != 2 || len(remote) != 1 {
		t.Fatalf("destroy changed watches: remote=%#v writes=%d diag=%v", remote, writes, deleted.Diagnostics)
	}
}

func TestWatchFoldersDetectsInvalidAndSilentlyIgnoredServerPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ignore bool
		bad    bool
	}{
		{name: "native ignored path", ignore: true},
		{name: "invalid server union", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := map[string]any{}
			c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
					_ = r.ParseForm()
					var patch map[string]any
					_ = json.Unmarshal([]byte(r.Form.Get("json")), &patch)
					if tc.ignore {
						remote = map[string]any{}
					} else {
						remote = map[string]any{"/bad": true}
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				b, _ := json.Marshal(map[string]any{"scan_dirs": remote})
				_, _ = w.Write(b)
			})
			r := &watchFoldersResource{client: c}
			s := watchFoldersSchema()
			state := watchFoldersTestState(t, s, configuredWatchFolders(t, map[string]client.WatchFolder{"/bad": {SaveMode: client.WatchFolderModeDefault}}))
			var create resource.CreateResponse
			create.State = tfsdk.State{Schema: s}
			r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}}, &create)
			if !create.Diagnostics.HasError() {
				t.Fatalf("expected post-write verification diagnostic, remote=%#v", remote)
			}
		})
	}
}

func TestWatchFoldersReadDetectsDriftAndCanonicalizesEqualPaths(t *testing.T) {
	remote := map[string]any{"/watch": "/watch"}
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]any{"scan_dirs": remote})
		_, _ = w.Write(b)
	})
	r := &watchFoldersResource{client: c}
	s := watchFoldersSchema()
	configured := map[string]client.WatchFolder{"/watch": {SaveMode: client.WatchFolderModeCustom, SavePath: "/watch"}}
	state := watchFoldersTestState(t, s, configuredWatchFolders(t, configured))
	var read resource.ReadResponse
	r.Read(context.Background(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var folders types.Map
	read.State.GetAttribute(context.Background(), path.Root("folders"), &folders)
	got, err := watchFoldersFromValue(folders)
	if err != nil || !reflect.DeepEqual(got, configured) {
		t.Fatalf("equal-path custom mode was not preserved: %#v %v", got, err)
	}

	remote = map[string]any{"/watch": 1}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	read.State.GetAttribute(context.Background(), path.Root("folders"), &folders)
	got, err = watchFoldersFromValue(folders)
	if err != nil || got["/watch"].SaveMode != client.WatchFolderModeDefault {
		t.Fatalf("drift was not refreshed: %#v %v", got, err)
	}
}

func TestWatchFoldersNullDoesNotWriteAndEmptyIsTypedEmptyMap(t *testing.T) {
	s := watchFoldersSchema()
	null := types.MapNull(watchFolderObjectType)
	empty := configuredWatchFolders(t, map[string]client.WatchFolder{})
	if !null.IsNull() || empty.IsNull() || len(empty.Elements()) != 0 {
		t.Fatalf("null and empty folders collapsed: null=%s empty=%s", null, empty)
	}
	state := watchFoldersTestState(t, s, null)
	var raw map[string]tftypes.Value
	if err := state.Raw.As(&raw); err != nil {
		t.Fatal(err)
	}
	if raw["folders"].IsNull() != true {
		t.Fatalf("null folders plan corrupted: %#v", raw["folders"])
	}
}
