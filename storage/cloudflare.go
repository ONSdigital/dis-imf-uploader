package storage

import (
	"context"
	"fmt"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/cloudflare/cloudflare-go"
)

// CloudflareClient provides methods for interacting with Cloudflare's API.
type CloudflareClient struct {
	client *cloudflare.API
	zoneID string
}

// NewCloudflare creates a new Cloudflare client with the provided
// configuration.
func NewCloudflare(cfg *config.Config) (*CloudflareClient, error) {
	if cfg.Token == "" {
		return &CloudflareClient{zoneID: cfg.ZoneID}, nil
	}

	api, err := cloudflare.NewWithAPIToken(cfg.Token)
	if err != nil {
		return nil, err
	}

	return &CloudflareClient{
		client: api,
		zoneID: cfg.ZoneID,
	}, nil
}

// PurgeCache purges the Cloudflare cache for the specified prefix.
func (c *CloudflareClient) PurgeCache(ctx context.Context, prefix string) error {
	if c.client == nil {
		return fmt.Errorf("cloudflare client not initialized")
	}

	resp, err := c.client.PurgeCache(
		ctx,
		c.zoneID,
		cloudflare.PurgeCacheRequest{
			Files: []string{prefix},
		},
	)

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("cloudflare purge failed")
	}

	return nil
}
