package client

import (
	"context"
	"net/http"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

type Category = openapi.Category

func (c *Client) Categories(ctx context.Context) (map[string]Category, error) {
	var cats map[string]Category
	b, err := c.do(ctx, "torrents/categories", true, func(ctx context.Context) (*http.Response, error) {
		return c.api.GetCategories(ctx)
	})
	if err != nil {
		return nil, err
	}
	err = decodeObject(b, &cats)
	return cats, err
}
func (c *Client) CreateCategory(ctx context.Context, cat Category) error {
	_, err := c.do(ctx, "torrents/createCategory", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.CreateCategoryWithFormdataBody(ctx, openapi.CreateCategoryFormdataRequestBody{Category: cat.Name, SavePath: &cat.SavePath})
	})
	return err
}
func (c *Client) EditCategory(ctx context.Context, cat Category) error {
	_, err := c.do(ctx, "torrents/editCategory", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.EditCategoryWithFormdataBody(ctx, openapi.EditCategoryFormdataRequestBody{Category: cat.Name, SavePath: &cat.SavePath})
	})
	return err
}
func (c *Client) DeleteCategory(ctx context.Context, name string) error {
	_, err := c.do(ctx, "torrents/removeCategories", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.RemoveCategoriesWithFormdataBody(ctx, openapi.RemoveCategoriesFormdataRequestBody{Categories: name})
	})
	return err
}
