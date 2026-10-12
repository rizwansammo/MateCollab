// Package objectstore is the S3-compatible storage abstraction (ADR-010). Code depends on the
// Store interface; the S3 implementation works with MinIO locally and any S3-compatible service
// in production. Object keys are always generated server-side from IDs.
package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
)

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
}

// DownloadOptions control the headers a presigned download URL forces on the response.
type DownloadOptions struct {
	Filename    string // Content-Disposition attachment filename
	ContentType string // response Content-Type (the detected type, never the declared one)
	Inline      bool   // only for safe preview types (FILES.md §6)
}

// Store is the storage interface used by the rest of the codebase.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Head(ctx context.Context, key string) (ObjectInfo, error)
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, keys ...string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration, o DownloadOptions) (string, error)
	Ping(ctx context.Context) error
}

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("objectstore: object not found")

// S3 implements Store over the S3 API.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewS3 builds the client. Presigning uses a second client bound to the public endpoint because
// SigV4 signatures include the host (FILES.md §2).
func NewS3(cfg config.StorageConfig) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("objectstore: endpoint and bucket are required")
	}
	base := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		// Only send/validate checksums when the API requires them: broadest S3 compatibility.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}
	mk := func(endpoint string) *s3.Client {
		return s3.NewFromConfig(base, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}
	public := cfg.PublicEndpoint
	if public == "" {
		public = cfg.Endpoint
	}
	return &S3{client: mk(cfg.Endpoint), presign: s3.NewPresignClient(mk(public)), bucket: cfg.Bucket}, nil
}

// Put uploads an object (server-side writes such as previews and processed avatars).
func (s *S3) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket, Key: &key, Body: body, ContentLength: aws.Int64(size), ContentType: aws.String(contentType),
	})
	return err
}

// Head returns object metadata.
func (s *S3) Head(ctx context.Context, key string) (ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: aws.ToString(out.ETag)}, nil
}

// Get streams an object; the caller closes the reader.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	return out.Body, ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: aws.ToString(out.ETag)}, nil
}

// Delete removes objects; missing objects are not an error.
func (s *S3) Delete(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: aws.String(k)}); err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

// PresignGet returns a short-lived download URL with forced response headers.
func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration, o DownloadOptions) (string, error) {
	in := &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}
	disposition := "attachment"
	if o.Inline {
		disposition = "inline"
	}
	if o.Filename != "" {
		disposition += "; filename*=UTF-8''" + url.PathEscape(o.Filename)
	}
	in.ResponseContentDisposition = aws.String(disposition)
	if o.ContentType != "" {
		in.ResponseContentType = aws.String(o.ContentType)
	}
	req, err := s.presign.PresignGetObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Ping checks that the bucket is reachable with the configured credentials.
func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	return err
}

// InitResult reports what Init could configure.
type InitResult struct {
	BucketCreated bool
	CORS          string // "applied" or the reason it was skipped
	Lifecycle     string
}

// Init creates the bucket if needed and applies CORS and lifecycle rules where the provider
// supports them (MinIO applies CORS server-wide instead of per bucket).
func (s *S3) Init(ctx context.Context, allowedOrigins []string) (InitResult, error) {
	var res InitResult
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket}); err != nil {
		if !isNotFound(err) {
			return res, fmt.Errorf("head bucket: %w", err)
		}
		if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket}); err != nil {
			return res, fmt.Errorf("create bucket: %w", err)
		}
		res.BucketCreated = true
	}

	_, err := s.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: &s.bucket,
		CORSConfiguration: &types.CORSConfiguration{CORSRules: []types.CORSRule{{
			AllowedOrigins: allowedOrigins,
			AllowedMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPost},
			AllowedHeaders: []string{"*"},
			ExposeHeaders:  []string{"ETag"},
			MaxAgeSeconds:  aws.Int32(3600),
		}}},
	})
	res.CORS = outcome(err)

	// Lifecycle: abort abandoned multipart uploads and expire scratch objects under tmp/. MinIO
	// rejects the multipart rule (it expires stale uploads itself), so fall back to expiry only.
	abortMultipart := types.LifecycleRule{
		ID: aws.String("abort-incomplete-multipart"), Status: types.ExpirationStatusEnabled,
		Filter:                         &types.LifecycleRuleFilter{Prefix: aws.String("")},
		AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{DaysAfterInitiation: aws.Int32(1)},
	}
	expireTmp := types.LifecycleRule{
		ID: aws.String("expire-tmp"), Status: types.ExpirationStatusEnabled,
		Filter:     &types.LifecycleRuleFilter{Prefix: aws.String("tmp/")},
		Expiration: &types.LifecycleExpiration{Days: aws.Int32(1)},
	}
	putLifecycle := func(rules ...types.LifecycleRule) error {
		_, err := s.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
			Bucket:                 &s.bucket,
			LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: rules},
		})
		return err
	}
	if err := putLifecycle(abortMultipart, expireTmp); err == nil {
		res.Lifecycle = "applied"
	} else if fallbackErr := putLifecycle(expireTmp); fallbackErr == nil {
		res.Lifecycle = "applied tmp/ expiry; multipart rule " + outcome(err)
	} else {
		res.Lifecycle = outcome(fallbackErr)
	}
	return res, nil
}

func outcome(err error) string {
	if err == nil {
		return "applied"
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return "skipped (" + apiErr.ErrorCode() + ")"
	}
	return "skipped (" + err.Error() + ")"
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return true
		}
	}
	var re interface{ HTTPStatusCode() int }
	if errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound {
		return true
	}
	return strings.Contains(err.Error(), "StatusCode: 404")
}
