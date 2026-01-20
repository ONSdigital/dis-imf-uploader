package storage

import (
	"context"
	"fmt"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/cloudflare/cloudflare-go"
)

type CloudflareClient struct {
	client *cloudflare.API
	zoneID string
}

func NewCloudflare(cfg *config.Config) (*CloudflareClient, error) {
	if cfg.CloudflareConfig.Token == "" {
		return &CloudflareClient{zoneID: cfg.CloudflareConfig.ZoneID}, nil
	}

	api, err := cloudflare.NewWithAPIToken(cfg.CloudflareConfig.Token)
	if err != nil {
		return nil, err
	}

	return &CloudflareClient{
		client: api,
		zoneID: cfg.CloudflareConfig.ZoneID,
	}, nil
}

func (c *CloudflareClient) PurgeCache(ctx context.Context, prefix string) error {
	if c.client == nil {
		return fmt.Errorf("cloudflare client not initialized")
	}

	resp, err := c.client.PurgeCache(ctx, c.zoneID,
		cloudflare.PurgeCacheRequest{
			Files: []string{prefix},
		})

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("cloudflare purge failed")
	}

	return nil
}
