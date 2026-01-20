package mongo

import (
	"context"
	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type DB struct {
	client *mongo.Client
	db     *mongo.Database
}

func New(cfg config.MongoConfig) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(
		"mongodb://"+cfg.ClusterEndpoint))
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
	// Email index for users
	userIndexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	d.db.Collection(config.UsersCollectionName).Indexes().CreateOne(ctx, userIndexModel)

	// Status index for uploads
	uploadStatusIndexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}},
	}
	d.db.Collection(config.UploadsCollectionName).Indexes().CreateOne(ctx, uploadStatusIndexModel)

	// TTL index for temp files
	ttlIndexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	d.db.Collection(config.UploadsCollectionName).Indexes().CreateOne(ctx, ttlIndexModel)

	return nil
}

func (d *DB) CreateUpload(ctx context.Context, upload *models.Upload) error {
	upload.ID = uuid.New().String()
	upload.UploadedAt = time.Now()
	upload.Status = models.StatusPending

	_, err := d.db.Collection(config.UploadsCollectionName).InsertOne(ctx, upload)
	return err
}

func (d *DB) GetUpload(ctx context.Context, id string) (*models.Upload, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var upload models.Upload
	err = d.db.Collection(config.UploadsCollectionName).FindOne(ctx,
		bson.M{"_id": objID}).Decode(&upload)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &upload, err
}

func (d *DB) GetPendingUploads(ctx context.Context) ([]*models.Upload, error) {
	cursor, err := d.db.Collection(config.UploadsCollectionName).Find(ctx,
		bson.M{"status": models.StatusPending})
	if err != nil {
		return nil, err
	}

	var uploads []*models.Upload
	err = cursor.All(ctx, &uploads)
	return uploads, err
}

func (d *DB) UpdateUploadApproved(ctx context.Context, id string,
	reviewedBy, s3Key, backupKey, invID string) error {

	objID, _ := primitive.ObjectIDFromHex(id)
	now := time.Now()

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":            models.StatusApproved,
			"reviewed_by":       reviewedBy,
			"reviewed_at":       now,
			"s3_key":            s3Key,
			"backup_s3_key":     backupKey,
			"cloudfront_inv_id": invID,
			"invocation_status": "InProgress",
		}})

	return err
}

func (d *DB) UpdateUploadRejected(ctx context.Context, id string,
	reviewedBy, reason string) error {

	objID, _ := primitive.ObjectIDFromHex(id)
	now := time.Now()

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":           models.StatusRejected,
			"reviewed_by":      reviewedBy,
			"reviewed_at":      now,
			"rejection_reason": reason,
		}})

	return err
}

func (d *DB) UpdateCloudFrontStatus(ctx context.Context, id string, status string) error {
	objID, _ := primitive.ObjectIDFromHex(id)

	_, err := d.db.Collection(config.UploadsCollectionName).UpdateOne(ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"invocation_status": status,
		}})

	return err
}

func (d *DB) SaveBackupMetadata(ctx context.Context, meta *models.BackupMetadata) error {
	meta.ID = uuid.New().String()
	_, err := d.db.Collection(config.BackupsCollectionName).InsertOne(ctx, meta)
	return err
}

func (d *DB) LogAudit(ctx context.Context, log *models.AuditLog) error {
	log.ID = uuid.New().String()
	_, err := d.db.Collection(config.AuditLogsCollectionName).InsertOne(ctx, log)
	return err
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := d.db.Collection(config.UsersCollectionName).FindOne(ctx,
		bson.M{"email": email}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (d *DB) CreateUser(ctx context.Context, user *models.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := d.db.Collection(config.UsersCollectionName).InsertOne(ctx, user)
	return err
}

func (d *DB) ListUploads(ctx context.Context, status string, page, pageSize int,
	sortBy, sortDir string) ([]*models.Upload, int64, error) {

	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}

	// Count total
	total, err := d.db.Collection(config.UploadsCollectionName).CountDocuments(ctx, filter)
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

	cursor, err := d.db.Collection(config.UploadsCollectionName).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	var uploads []*models.Upload
	if err = cursor.All(ctx, &uploads); err != nil {
		return nil, 0, err
	}

	return uploads, total, nil
}

func (d *DB) GetUser(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := d.db.Collection(config.UsersCollectionName).FindOne(ctx,
		bson.M{"_id": id}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (d *DB) UpdateUser(ctx context.Context, id string, name string, role models.UserRole) error {
	update := bson.M{"updated_at": time.Now()}
	if name != "" {
		update["name"] = name
	}
	if role != "" {
		update["role"] = role
	}

	_, err := d.db.Collection(config.UsersCollectionName).UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": update})
	return err
}

func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.db.Collection(config.UsersCollectionName).DeleteOne(ctx,
		bson.M{"_id": id})
	return err
}

func (d *DB) ListAuditLogs(ctx context.Context, uploadID, action, userEmail string,
	page, pageSize int) ([]*models.AuditLog, int64, error) {

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

	total, err := d.db.Collection(config.AuditLogsCollectionName).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := d.db.Collection(config.AuditLogsCollectionName).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	var logs []*models.AuditLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (d *DB) HealthCheck(ctx context.Context) error {
	return d.client.Ping(ctx, nil)
}

func (d *DB) Close() error {
	return d.client.Disconnect(context.Background())
}
