package provider

import (
	"context"
	"errors"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
	"strings"
)

type categoryResource struct{ client *client.Client }
type categoryModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	SavePath types.String `tfsdk:"save_path"`
}

func (r *categoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_category"
}
func (r *categoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage one download category. Import by name. Rename replaces the category. Destroy reassigns torrents to the parent category; TMM may relocate data. It does not delete torrents or files. Delete is refused while descendant categories exist; remove them first. Avoid concurrent hierarchy mutations.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Category name, used as the resource ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":      schema.StringAttribute{Required: true, MarkdownDescription: "Category name. Slash-separated subcategories require existing ancestors. Reference the parent resource name to order lifecycle operations. Changing this forces replacement.", Validators: []validator.String{categoryNameValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"save_path": schema.StringAttribute{Required: true, MarkdownDescription: "Save path on the server. An empty path resolves to the category basename beneath its parent effective path (or the default save path for a top-level category). A nonempty relative path resolves beneath the global default save path. Absolute paths are used directly. Use normalized paths."},
		}}
}
func (r *categoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}
func (r *categoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m categoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Upstream implicitly creates missing ancestors. Refuse to create objects
	// Terraform does not own; an explicit parent reference also orders destroy.
	if strings.Contains(m.Name.ValueString(), "/") {
		cats, err := r.client.Categories(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Cannot check category ancestors", err.Error())
			return
		}
		parts := strings.Split(m.Name.ValueString(), "/")
		for i := 1; i < len(parts); i++ {
			if _, exists := cats[strings.Join(parts[:i], "/")]; !exists {
				resp.Diagnostics.AddAttributeError(path.Root("name"), "Missing ancestor category", "Create or import every ancestor first. Reference the parent resource name in the child name so Terraform orders creation and deletion. No category was created.")
				return
			}
		}
	}
	err := r.client.CreateCategory(ctx, client.Category{Name: m.Name.ValueString(), SavePath: m.SavePath.ValueString()})
	if err != nil {
		var api *client.HTTPError
		if errors.As(err, &api) && api.StatusCode == 409 {
			resp.Diagnostics.AddError("Category already exists or name is invalid", "If this category exists, import it with: terraform import <resource-address> <category-name>. Creation did not adopt it.")
		} else {
			resp.Diagnostics.AddError("Cannot create category", err.Error())
		}
		return
	}
	m.ID = m.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	cats, err := r.client.Categories(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read created category", err.Error())
		return
	}
	cat, ok := cats[m.Name.ValueString()]
	if !ok {
		resp.Diagnostics.AddError("Created category missing", "The server did not return the created category.")
		return
	}
	if cat.SavePath != m.SavePath.ValueString() {
		resp.Diagnostics.AddAttributeError(path.Root("save_path"), "Category path normalized", "Use the server-normalized path in configuration.")
	}
}
func (r *categoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m categoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cats, err := r.client.Categories(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read category", err.Error())
		return
	}
	cat, ok := cats[m.ID.ValueString()]
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Name = m.ID
	m.SavePath = types.StringValue(cat.SavePath)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *categoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m categoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.EditCategory(ctx, client.Category{Name: m.Name.ValueString(), SavePath: m.SavePath.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Cannot update category", err.Error())
		return
	}
	m.ID = m.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	cats, err := r.client.Categories(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read updated category", err.Error())
		return
	}
	cat, ok := cats[m.Name.ValueString()]
	if !ok {
		resp.Diagnostics.AddError("Updated category missing", "Category was removed concurrently.")
		return
	}
	if cat.SavePath != m.SavePath.ValueString() {
		resp.Diagnostics.AddAttributeError(path.Root("save_path"), "Category path normalized", "Use the server-normalized path in configuration.")
	}
}
func (r *categoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m categoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cats, err := r.client.Categories(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot check category descendants", err.Error())
		return
	}
	for name := range cats {
		if strings.HasPrefix(name, m.ID.ValueString()+"/") {
			resp.Diagnostics.AddError("Category has descendants", "qBittorrent recursively removes subcategories. Remove descendants first, then retry this deletion.")
			return
		}
	}
	if err = r.client.DeleteCategory(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Cannot delete category", err.Error())
	}
}
func (r *categoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !validCategoryName(req.ID) {
		resp.Diagnostics.AddError("Invalid import ID", "Use the existing category name.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
