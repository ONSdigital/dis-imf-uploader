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

type CloudFrontClient struct {
	client *cloudfront.Client
}

func NewCloudFront(cfg *config.Config) (*CloudFrontClient, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(),
		awsConfig.WithRegion(cfg.S3Config.Region))
	if err != nil {
		return nil, err
	}

	return &CloudFrontClient{
		client: cloudfront.NewFromConfig(awsCfg),
	}, nil
}

func (c *CloudFrontClient) InvalidateCache(ctx context.Context,
	distributionID string, paths []string) (string, error) {

	if distributionID == "" {
		return "", fmt.Errorf("distribution ID is required")
	}

	items := make([]string, len(paths))
	copy(items, paths)

	invalidation, err := c.client.CreateInvalidation(ctx,
		&cloudfront.CreateInvalidationInput{
			DistributionId: aws.String(distributionID),
			InvalidationBatch: &types.InvalidationBatch{
				CallerReference: aws.String(fmt.Sprintf("%d", time.Now().UnixNano())),
				Paths: &types.Paths{
					Quantity: aws.Int32(int32(len(items))),
					Items:    items,
				},
			},
		})

	if err != nil {
		return "", err
	}

	return *invalidation.Invalidation.Id, nil
}

func (c *CloudFrontClient) GetInvalidationStatus(ctx context.Context,
	distributionID, invID string) (string, error) {

	inv, err := c.client.GetInvalidation(ctx,
		&cloudfront.GetInvalidationInput{
			DistributionId: aws.String(distributionID),
			Id:             aws.String(invID),
		})

	if err != nil {
		return "", err
	}

	return *inv.Invalidation.Status, nil
}
