package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"sip-happens/pkg/config"
	"sip-happens/pkg/domain"
)

// ProductImages stores catalog product images in S3-compatible object storage.
type ProductImages interface {
	UploadProductImage(ctx context.Context, productID domain.ProductID, sku, contentType string, r io.Reader, size int64) (publicURL string, err error)
}

type MinIOProductImages struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func NewMinIOProductImages(cfg config.MinIO) (*MinIOProductImages, error) {
	if !cfg.Enabled() {
		return nil, fmt.Errorf("minio is not configured")
	}
	if cfg.PublicURL == "" {
		return nil, fmt.Errorf("MINIO_PUBLIC_URL is required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &MinIOProductImages{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
	}, nil
}

func NewProductImagesFromEnv() (ProductImages, error) {
	cfg := config.LoadMinIO()
	if !cfg.Enabled() {
		return nil, nil
	}
	return NewMinIOProductImages(cfg)
}

func (s *MinIOProductImages) UploadProductImage(ctx context.Context, productID domain.ProductID, sku, contentType string, r io.Reader, size int64) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("object store not configured")
	}
	ext, err := extForContentType(contentType)
	if err != nil {
		return "", err
	}
	safeSKU := sanitizeObjectKeyPart(sku)
	if safeSKU == "" {
		safeSKU = "product"
	}
	objectKey := path.Join("products", string(productID), safeSKU+ext)

	_, err = s.client.PutObject(ctx, s.bucket, objectKey, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}
	return s.publicURL + "/" + objectKey, nil
}

func extForContentType(ct string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(ct)) {
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", domain.ErrInvalidInput
	}
}

func sanitizeObjectKeyPart(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DetectImageContentType peeks the first 512 bytes for image/* validation.
func DetectImageContentType(r io.Reader) (contentType string, body io.Reader, err error) {
	var head [512]byte
	n, _ := io.ReadFull(r, head[:])
	ct := http.DetectContentType(head[:n])
	switch ct {
	case "image/jpeg", "image/png", "image/webp":
		return ct, io.MultiReader(bytes.NewReader(head[:n]), r), nil
	default:
		return "", nil, domain.ErrInvalidInput
	}
}
