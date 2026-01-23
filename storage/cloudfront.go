package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// CloudFrontClient provides methods for interacting with AWS CloudFront.
type CloudFrontClient struct {
	client *cloudfront.Client
}

// NewCloudFront creates a new CloudFront client with the provided
// configuration.
func NewCloudFront(cfg *config.Config) (*CloudFrontClient, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(
		context.Background(),
		awsConfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	return &CloudFrontClient{
		client: cloudfront.NewFromConfig(awsCfg),
	}, nil
}

// InvalidateCache creates a cache invalidation for the specified paths and
// returns the invalidation ID.
func (c *CloudFrontClient) InvalidateCache(ctx context.Context, distributionID string, paths []string) (string, error) {
	if distributionID == "" {
		return "", fmt.Errorf("distribution ID is required")
	}

	items := make([]string, len(paths))
	copy(items, paths)

	invalidation, err := c.client.CreateInvalidation(
		ctx,
		&cloudfront.CreateInvalidationInput{
			DistributionId: aws.String(distributionID),
			InvalidationBatch: &types.InvalidationBatch{
				CallerReference: aws.String(
					fmt.Sprintf("%d", time.Now().UnixNano()),
				),
				Paths: &types.Paths{
					// #nosec G115 - len() result is always safe to convert
					Quantity: aws.Int32(int32(len(items))),
					Items:    items,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return *invalidation.Invalidation.Id, nil
}

// GetInvalidationStatus retrieves the status of a CloudFront cache
// invalidation.
func (c *CloudFrontClient) GetInvalidationStatus(ctx context.Context, distributionID, invID string) (string, error) {
	inv, err := c.client.GetInvalidation(
		ctx,
		&cloudfront.GetInvalidationInput{
			DistributionId: aws.String(distributionID),
			Id:             aws.String(invID),
		},
	)

	if err != nil {
		return "", err
	}

	return *inv.Invalidation.Status, nil
}
