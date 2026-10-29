package client

import (
	"context"
	"net/http"
	"net/url"
)

type Category struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
}

func (c *Client) Categories(ctx context.Context) (map[string]Category, error) {
	var cats map[string]Category
	b, err := c.do(ctx, http.MethodGet, "torrents/categories", nil)
	if err != nil {
		return nil, err
	}
	err = decodeObject(b, &cats)
	return cats, err
}
func (c *Client) CreateCategory(ctx context.Context, cat Category) error {
	return c.writeCategory(ctx, "createCategory", cat)
}
func (c *Client) EditCategory(ctx context.Context, cat Category) error {
	return c.writeCategory(ctx, "editCategory", cat)
}
func (c *Client) writeCategory(ctx context.Context, action string, cat Category) error {
	_, err := c.do(ctx, http.MethodPost, "torrents/"+action, url.Values{"category": {cat.Name}, "savePath": {cat.SavePath}})
	return err
}
func (c *Client) DeleteCategory(ctx context.Context, name string) error {
	_, err := c.do(ctx, http.MethodPost, "torrents/removeCategories", url.Values{"categories": {name}})
	return err
}
