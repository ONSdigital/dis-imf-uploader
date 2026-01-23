package sdk_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ONSdigital/dis-imf-uploader/sdk"
)

func Example_uploadWorkflow() {
	// Initialize the SDK client
	client := sdk.NewClient(
		"http://localhost:30200",
		"your-auth-token",
		"uploader@example.com",
	)

	ctx := context.Background()

	// Read file to upload
	fileData, err := os.ReadFile("document.pdf")
	if err != nil {
		log.Fatal(err)
	}

	// 1. Upload file for review
	uploadResp, err := client.UploadFile(ctx, "document.pdf", fileData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Upload ID: %s, Status: %s\n", uploadResp.ID, uploadResp.Status)

	// 2. Check upload status
	upload, err := client.GetUploadStatus(ctx, uploadResp.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Current status: %s\n", upload.Status)

	// 3. List all pending uploads (as reviewer)
	reviewerClient := sdk.NewClient(
		"http://localhost:30200",
		"your-auth-token",
		"reviewer@example.com",
	)

	uploads, err := reviewerClient.ListUploads(ctx, "pending", 1, 20)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Pending uploads: %d\n", uploads.Total)

	// 4. Approve the upload (as reviewer with permissions)
	approveResp, err := reviewerClient.ApproveUpload(ctx, uploadResp.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Approved! S3 Key: %s, CloudFront Inv ID: %s\n",
		approveResp.S3Key, approveResp.CloudFrontInvID)

	// 5. Or reject with reason
	// err = reviewerClient.RejectUpload(ctx, uploadResp.ID, "Incorrect format")
	// if err != nil {
	// 	log.Fatal(err)
	// }
}

func Example_auditLogs() {
	client := sdk.NewClient(
		"http://localhost:30200",
		"your-auth-token",
		"admin@example.com",
	)

	ctx := context.Background()

	// List audit logs for a specific upload
	logs, err := client.ListAuditLogs(ctx, "upload-id-123", "", "", 1, 50)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total logs: %d\n", logs.Total)
	for _, log := range logs.Logs {
		fmt.Printf("[%s] %s by %s: %s\n",
			log.Timestamp.Format("2006-01-02 15:04:05"),
			log.Action,
			log.UserID,
			log.Status,
		)
	}
}

func Example_healthCheck() {
	client := sdk.NewClient(
		"http://localhost:30200",
		"",
		"",
	)

	ctx := context.Background()

	health, err := client.HealthCheck(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Service status: %s\n", health.Status)
	fmt.Printf("Version: %s\n", health.Version)
	for dep, status := range health.Dependencies {
		fmt.Printf("  %s: %s\n", dep, status)
	}
}
