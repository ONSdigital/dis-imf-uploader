package mongo

import (
	"context"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:generate moq -out mock/store.go -pkg mock . Store

// Store defines the interface for database operations
type Store interface {
	CreateUpload(ctx context.Context, upload *models.Upload) error
	GetUpload(ctx context.Context, id string) (*models.Upload, error)
	GetPendingUploads(ctx context.Context) ([]*models.Upload, error)
	UpdateUploadApproved(ctx context.Context, id string, reviewedBy, s3Key, backupKey, invID string) error
	UpdateUploadRejected(ctx context.Context, id string, reviewedBy, reason string) error
	UpdateCloudFrontStatus(ctx context.Context, id, status string) error
	SaveBackupMetadata(ctx context.Context, meta *models.BackupMetadata) error
	LogAudit(ctx context.Context, log *models.AuditLog) error
	ListUploads(ctx context.Context, status string, page, pageSize int, sortBy, sortDir string) ([]*models.Upload, int64, error)
	ListAuditLogs(ctx context.Context, uploadID, action, userEmail string, page, pageSize int) ([]*models.AuditLog, int64, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

// DB represents a MongoDB database connection and provides methods for
// interacting with the database.
type DB struct {
	client *mongo.Client
	db     *mongo.Database
}

// New creates a new MongoDB database connection and initializes indexes.
func New(cfg config.MongoConfig) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI("mongodb://"+cfg.ClusterEndpoint),
	)
	if err != nil {
		return nil, err
	}

	database := client.Database(cfg.Database)

	db := &DB{
		client: client,
		db:     database,
	}

	if err := db.createIndexes(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func (d *DB) createIndexes(ctx context.Context) error {
	// Status index for uploads
	uploadStatusIndexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}},
	}
	_, _ = d.db.Collection(config.UploadsCollectionName).
		Indexes().
		CreateOne(ctx, uploadStatusIndexModel)

	// TTL index for temp files
	ttlIndexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	_, _ = d.db.Collection(config.UploadsCollectionName).
		Indexes().
		CreateOne(ctx, ttlIndexModel)

	return nil
}

// CreateUpload creates a new upload record in the database.
func (d *DB) CreateUpload(ctx context.Context, upload *models.Upload) error {
	upload.ID = uuid.New().String()
	upload.UploadedAt = time.Now()
	upload.Status = models.StatusPending

	_, err := d.db.Collection(config.UploadsCollectionName).
		InsertOne(ctx, upload)
	return err
}

// GetUpload retrieves an upload record by ID.
func (d *DB) GetUpload(ctx context.Context, id string) (*models.Upload, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var upload models.Upload
	err = d.db.Collection(config.UploadsCollectionName).
		FindOne(ctx, bson.M{"_id": objID}).
		Decode(&upload)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &upload, err
}

// GetPendingUploads retrieves all uploads with pending status.
func (d *DB) GetPendingUploads(ctx context.Context) ([]*models.Upload, error) {
	cursor, err := d.db.Collection(config.UploadsCollectionName).
		Find(ctx, bson.M{"status": models.StatusPending})
	if err != nil {
		return nil, err
	}

	var uploads []*models.Upload
	err = cursor.All(ctx, &uploads)
	return uploads, err
}

// UpdateUploadApproved updates an upload record to approved status.
func (d *DB) UpdateUploadApproved(ctx context.Context, id, reviewedBy, s3Key, backupKey, invID string) error {
	objID, _ := primitive.ObjectIDFromHex(id)
	now := time.Now()

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":            models.StatusApproved,
			"reviewed_by":       reviewedBy,
			"reviewed_at":       now,
			"s3_key":            s3Key,
			"backup_s3_key":     backupKey,
			"cloudfront_inv_id": invID,
			"invocation_status": "InProgress",
		}},
	)

	return err
}

// UpdateUploadRejected updates an upload record to rejected status.
func (d *DB) UpdateUploadRejected(ctx context.Context, id, reviewedBy, reason string) error {
	objID, _ := primitive.ObjectIDFromHex(id)
	now := time.Now()

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":           models.StatusRejected,
			"reviewed_by":      reviewedBy,
			"reviewed_at":      now,
			"rejection_reason": reason,
		}},
	)

	return err
}

// UpdateCloudFrontStatus updates the CloudFront invocation status for an
// upload.
func (d *DB) UpdateCloudFrontStatus(ctx context.Context, id, status string) error {
	objID, _ := primitive.ObjectIDFromHex(id)

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"invocation_status": status,
		}},
	)

	return err
}

// SaveBackupMetadata saves backup metadata to the database.
func (d *DB) SaveBackupMetadata(ctx context.Context, meta *models.BackupMetadata) error {
	meta.ID = uuid.New().String()
	_, err := d.db.Collection(config.BackupsCollectionName).
		InsertOne(ctx, meta)
	return err
}

// LogAudit creates an audit log entry in the database.
func (d *DB) LogAudit(ctx context.Context, log *models.AuditLog) error {
	log.ID = uuid.New().String()
	_, err := d.db.Collection(config.AuditLogsCollectionName).
		InsertOne(ctx, log)
	return err
}

// ListUploads retrieves a paginated list of uploads with optional filtering
// and sorting.
func (d *DB) ListUploads(ctx context.Context, status string, page, pageSize int, sortBy, sortDir string) ([]*models.Upload, int64, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}

	// Count total
	total, err := d.db.Collection(config.UploadsCollectionName).
		CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Build sort
	sortOrder := 1
	if sortDir == "desc" {
		sortOrder = -1
	}
	if sortBy == "" {
		sortBy = "uploaded_at"
		sortOrder = -1
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: sortBy, Value: sortOrder}})

	cursor, err := d.db.Collection(config.UploadsCollectionName).
		Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	var uploads []*models.Upload
	err = cursor.All(ctx, &uploads)
	if err != nil {
		return nil, 0, err
	}

	return uploads, total, nil
}

// ListAuditLogs retrieves a paginated list of audit logs with optional
// filtering.
func (d *DB) ListAuditLogs(ctx context.Context, uploadID, action, userEmail string, page, pageSize int) ([]*models.AuditLog, int64, error) {
	filter := bson.M{}
	if uploadID != "" {
		filter["upload_id"] = uploadID
	}
	if action != "" {
		filter["action"] = action
	}
	if userEmail != "" {
		filter["user_email"] = userEmail
	}

	total, err := d.db.Collection(config.AuditLogsCollectionName).
		CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := d.db.Collection(config.AuditLogsCollectionName).
		Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	var logs []*models.AuditLog
	err = cursor.All(ctx, &logs)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// HealthCheck checks the database connection health.
func (d *DB) HealthCheck(ctx context.Context) error {
	return d.client.Ping(ctx, nil)
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.client.Disconnect(context.Background())
}
