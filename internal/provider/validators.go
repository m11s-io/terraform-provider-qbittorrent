package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"strings"
)

type numberValidator struct{ port bool }

func (v numberValidator) Description(context.Context) string {
	if v.port {
		return "Port must be between 1 and 65535."
	}
	return "Rate must be 0 or a multiple of 1024 between 1024 and 2147482624 bytes/s."
}
func (v numberValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v numberValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := req.ConfigValue.ValueInt64()
	valid := n >= 0 && n <= 2147482624 && n%1024 == 0
	if v.port {
		valid = n >= 1 && n <= 65535
	}
	if !valid {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid numeric preference", v.Description(ctx))
	}
}

type categoryNameValidator struct{}

func (categoryNameValidator) Description(context.Context) string {
	return "Use a nonempty category name without backslashes, newlines, leading/trailing slash or repeated slash."
}
func (v categoryNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v categoryNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !validCategoryName(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid category name", v.Description(ctx))
	}
}
func validCategoryName(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\\\r\n\x00") && !strings.HasPrefix(s, "/") && !strings.HasSuffix(s, "/") && !strings.Contains(s, "//")
}
