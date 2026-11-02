package cards

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const cacheControl = "public, max-age=31536000, immutable"

// Uploader puts card PNGs in an R2 bucket under content-addressed keys.
type Uploader struct {
	S3         *s3.Client
	Bucket     string
	PublicBase string
}

// NewUploader points an S3 client at the R2 endpoint (path-style, region auto).
func NewUploader(endpoint, bucket, accessKey, secretKey, publicBase string) *Uploader {
	c := s3.New(s3.Options{
		Region:                     "auto",
		BaseEndpoint:               aws.String(endpoint),
		UsePathStyle:               true,
		Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &Uploader{S3: c, Bucket: bucket, PublicBase: strings.TrimRight(publicBase, "/")}
}

// Put stores png at cards/<sha256>.png and returns its public URL and the hash.
func (u *Uploader) Put(ctx context.Context, png []byte) (url, sha string, err error) {
	sum := sha256.Sum256(png)
	sha = hex.EncodeToString(sum[:])
	key := "cards/" + sha + ".png"
	_, err = u.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(u.Bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(png),
		ContentType:  aws.String("image/png"),
		CacheControl: aws.String(cacheControl),
	})
	if err != nil {
		return "", "", fmt.Errorf("put %s: %w", key, err)
	}
	return strings.TrimRight(u.PublicBase, "/") + "/" + key, sha, nil
}
