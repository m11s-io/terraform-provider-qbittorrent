package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

type watchFoldersResource struct{ client *client.Client }

func (r *watchFoldersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_watch_folders"
}

func (r *watchFoldersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Singleton authoritative ownership of qBittorrent's complete watched-folder map. Import ID: application. `folders` replaces the whole server map; an explicit empty map clears it, while omitting or removing `folders` relinquishes ownership without changing watches. Destroy preserves server watches. Import reads and adopts the complete current map. Paths refer to the qBittorrent server. qBittorrent may silently ignore invalid paths, so configured maps are verified against the normalized server response. This only configures watches; acceptance uses empty directories and does not add torrents or remove files.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Singleton ID: application.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"folders": schema.MapNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Complete watched-folder map keyed by server paths. Modes are `default`, `watch_folder`, or `custom`; `save_path` is required only for `custom`.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"save_mode": schema.StringAttribute{Required: true, MarkdownDescription: "Use the default save path, save into the watched folder, or use a custom save path."},
					"save_path": schema.StringAttribute{Optional: true, MarkdownDescription: "Custom save path on the qBittorrent server; required when `save_mode = \"custom\"`."},
				}},
			},
		},
	}
}

func (r *watchFoldersResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var folders types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("folders"), &folders)...)
	if resp.Diagnostics.HasError() || folders.IsNull() || folders.IsUnknown() {
		return
	}
	values := folders.Elements()
	for watchedPath, value := range values {
		p := path.Root("folders").AtMapKey(watchedPath)
		if watchedPath == "" {
			resp.Diagnostics.AddAttributeError(p, "Invalid watched folder path", "Watched-folder paths must not be empty.")
		}
		obj, ok := value.(types.Object)
		if !ok {
			resp.Diagnostics.AddAttributeError(p, "Invalid watched folder", "Each folder must be a known object.")
			continue
		}
		if obj.IsNull() || obj.IsUnknown() {
			continue
		}
		attrs := obj.Attributes()
		mode, modeOK := attrs["save_mode"].(types.String)
		savePath, pathOK := attrs["save_path"].(types.String)
		if !modeOK || !pathOK {
			resp.Diagnostics.AddAttributeError(p, "Invalid watched folder", "Each folder must contain save_mode and save_path attributes.")
			continue
		}
		if mode.IsUnknown() {
			continue
		}
		if mode.IsNull() {
			resp.Diagnostics.AddAttributeError(p.AtName("save_mode"), "Missing save mode", "Configure save_mode as default, watch_folder, or custom.")
			continue
		}
		switch client.WatchFolderMode(mode.ValueString()) {
		case client.WatchFolderModeDefault, client.WatchFolderModeWatchFolder:
			if !savePath.IsNull() && !savePath.IsUnknown() && savePath.ValueString() != "" {
				resp.Diagnostics.AddAttributeError(p.AtName("save_path"), "Unexpected custom save path", "Configure save_path only when save_mode is custom.")
			}
		case client.WatchFolderModeCustom:
			if savePath.IsUnknown() {
				continue
			}
			if savePath.IsNull() || savePath.ValueString() == "" {
				resp.Diagnostics.AddAttributeError(p.AtName("save_path"), "Missing custom save path", "Configure a non-empty save_path when save_mode is custom.")
			}
		default:
			resp.Diagnostics.AddAttributeError(p.AtName("save_mode"), "Invalid save mode", "Use default, watch_folder, or custom.")
		}
	}
}

func (r *watchFoldersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}

func (r *watchFoldersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var folders types.Map
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("folders"), &folders)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
	r.apply(ctx, folders, &resp.State, &resp.Diagnostics)
}

func (r *watchFoldersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var folders types.Map
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("folders"), &folders)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
	r.apply(ctx, folders, &resp.State, &resp.Diagnostics)
}

func (r *watchFoldersResource) apply(ctx context.Context, planned types.Map, state *tfsdk.State, d *diag.Diagnostics) {
	if planned.IsNull() {
		return // Null relinquishes ownership and must not emit scan_dirs.
	}
	folders, err := watchFoldersFromValue(planned)
	if err != nil {
		d.AddAttributeError(path.Root("folders"), "Invalid watched folders", err.Error())
		return
	}
	if err := client.ValidateWatchFolders(folders); err != nil {
		d.AddAttributeError(path.Root("folders"), "Invalid watched folders", err.Error())
		return
	}
	current, err := r.client.WatchFolders(ctx)
	if err != nil {
		d.AddError("Cannot verify current watched folders", "The existing scan_dirs value is missing or invalid. No update was sent.")
		return
	}
	for watchedPath, desired := range folders {
		prior, exists := current[watchedPath]
		if exists && desired.SaveMode == client.WatchFolderModeDefault && prior.SaveMode != client.WatchFolderModeDefault {
			observed, valueErr := watchFoldersValue(current)
			if valueErr == nil {
				d.Append(state.SetAttribute(ctx, path.Root("folders"), observed)...)
			}
			d.AddAttributeError(path.Root("folders").AtMapKey(watchedPath), "Cannot switch watched folder to default in one apply", "The legacy scan_dirs API preserves the previous save path for integer 1. Remove this folder from the authoritative map in one apply, then add it again with save_mode = default in a later apply. No update was sent.")
			return
		}
	}
	if err := r.client.SetWatchFolders(ctx, folders); err != nil {
		d.AddError("Cannot replace watched folders", err.Error())
		return
	}
	actual, err := r.client.WatchFolders(ctx)
	if err != nil {
		d.AddError("Cannot verify watched folders", "The server response is missing or contains invalid scan_dirs values. No files were changed by this provider, but the watch-folder update outcome must be checked on the server.")
		return
	}
	if !watchFoldersEquivalent(folders, actual) {
		observed, valueErr := watchFoldersValue(actual)
		if valueErr == nil {
			d.Append(state.SetAttribute(ctx, path.Root("folders"), observed)...)
		}
		d.AddAttributeError(path.Root("folders"), "Watched folders rejected or normalized", "qBittorrent returned a different watched-folder map after replacement. Check server paths and retry with the server-normalized folders; the full map may have been partially ignored.")
		return
	}
}

func (r *watchFoldersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	var folders types.Map
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("folders"), &folders)...)
	if resp.Diagnostics.HasError() || folders.IsNull() {
		return // Unconfigured ownership stays null on import/update refresh.
	}
	actual, err := r.client.WatchFolders(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read watched folders", "The server response is missing or contains invalid scan_dirs values.")
		return
	}
	prior, err := watchFoldersFromValue(folders)
	if err != nil {
		resp.Diagnostics.AddError("Invalid stored watched folders", "The stored folders value cannot be decoded.")
		return
	}
	if watchFoldersEquivalent(prior, actual) {
		return // Preserve a configured custom path equal to its watched path.
	}
	updated, err := watchFoldersValue(actual)
	if err != nil {
		resp.Diagnostics.AddError("Cannot refresh watched folders", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folders"), updated)...)
}

func (r *watchFoldersResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Destroy relinquishes ownership and intentionally leaves server watches intact.
}

func (r *watchFoldersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "application" {
		resp.Diagnostics.AddError("Invalid import ID", "Watched-folders resource import ID must be application.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	actual, err := r.client.WatchFolders(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot import watched folders", "The server response is missing or contains invalid scan_dirs values.")
		return
	}
	value, err := watchFoldersValue(actual)
	if err != nil {
		resp.Diagnostics.AddError("Cannot import watched folders", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folders"), value)...)
}

func watchFoldersFromValue(value types.Map) (map[string]client.WatchFolder, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, fmt.Errorf("folders must be known when configured")
	}
	values := value.Elements()
	folders := make(map[string]client.WatchFolder, len(values))
	for watchedPath, value := range values {
		obj, ok := value.(types.Object)
		if !ok || obj.IsNull() || obj.IsUnknown() {
			return nil, fmt.Errorf("watched folder %q must be a known object", watchedPath)
		}
		attrs := obj.Attributes()
		mode, okMode := attrs["save_mode"].(types.String)
		savePath, okPath := attrs["save_path"].(types.String)
		if !okMode || !okPath || mode.IsNull() || mode.IsUnknown() || savePath.IsUnknown() {
			return nil, fmt.Errorf("watched folder %q has invalid fields", watchedPath)
		}
		folder := client.WatchFolder{SaveMode: client.WatchFolderMode(mode.ValueString())}
		if !savePath.IsNull() {
			folder.SavePath = savePath.ValueString()
		}
		folders[watchedPath] = folder
	}
	return folders, nil
}

func watchFoldersValue(folders map[string]client.WatchFolder) (types.Map, error) {
	values := make(map[string]attr.Value, len(folders))
	for watchedPath, folder := range folders {
		attrs := map[string]attr.Value{"save_mode": types.StringValue(string(folder.SaveMode)), "save_path": types.StringNull()}
		if folder.SavePath != "" {
			attrs["save_path"] = types.StringValue(folder.SavePath)
		}
		obj, diagnostics := types.ObjectValue(watchFolderAttrTypes, attrs)
		if diagnostics.HasError() {
			return types.MapNull(watchFolderObjectType), fmt.Errorf("cannot encode watched folder %q", watchedPath)
		}
		values[watchedPath] = obj
	}
	result, diagnostics := types.MapValue(watchFolderObjectType, values)
	if diagnostics.HasError() {
		return types.MapNull(watchFolderObjectType), fmt.Errorf("cannot encode watched folders")
	}
	return result, nil
}

func watchFoldersEquivalent(left, right map[string]client.WatchFolder) bool {
	l, err := client.EncodeWatchFolders(left)
	if err != nil {
		return false
	}
	r, err := client.EncodeWatchFolders(right)
	if err != nil || len(l) != len(r) {
		return false
	}
	for key, value := range l {
		if r[key] != value {
			return false
		}
	}
	return true
}

var (
	watchFolderAttrTypes  = map[string]attr.Type{"save_mode": types.StringType, "save_path": types.StringType}
	watchFolderObjectType = types.ObjectType{AttrTypes: watchFolderAttrTypes}
)

var _ resource.Resource = (*watchFoldersResource)(nil)
var _ resource.ResourceWithImportState = (*watchFoldersResource)(nil)
