package config

import (
	"os"
	"strings"
)

// MinIO holds optional object storage settings for product images.
type MinIO struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	// PublicURL is the browser-reachable base URL (e.g. http://localhost:9000/product-images).
	PublicURL string
	UseSSL    bool
}

func LoadMinIO() MinIO {
	return MinIO{
		Endpoint:  strings.TrimSpace(os.Getenv("MINIO_ENDPOINT")),
		AccessKey: strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY")),
		SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		Bucket:    getenvDefault("MINIO_BUCKET", "product-images"),
		PublicURL: trimRightSlash(getenvDefault("MINIO_PUBLIC_URL", "")),
		UseSSL:    getenvBoolDefault("MINIO_USE_SSL", false),
	}
}

func (m MinIO) Enabled() bool {
	return m.Endpoint != "" && m.AccessKey != "" && m.SecretKey != "" && m.Bucket != ""
}

func trimRightSlash(s string) string {
	s = strings.TrimSpace(s)
	for len(s) > 1 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
