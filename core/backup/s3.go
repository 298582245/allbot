package backup

import (
	"context"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Uploader struct{}

func NewS3Uploader() *S3Uploader {
	return &S3Uploader{}
}

func ValidateS3BackupSettings(settings config.OSSBackupSettings) error {
	if !settings.Enabled {
		return nil
	}
	if strings.TrimSpace(settings.Bucket) == "" {
		return fmt.Errorf("S3 存储桶不能为空")
	}
	if (strings.TrimSpace(settings.AccessKey) == "") != (strings.TrimSpace(settings.SecretKey) == "") {
		return fmt.Errorf("S3 Access Key 和 Secret Key 必须同时填写")
	}
	if endpoint := strings.TrimSpace(settings.Endpoint); endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("S3 Endpoint 必须是 http 或 https 地址")
		}
	}
	switch normalizeAddressingStyle(settings.AddressingStyle) {
	case "auto", "path", "virtual":
		return nil
	default:
		return fmt.Errorf("S3 寻址方式无效")
	}
}

func (u *S3Uploader) Upload(ctx context.Context, file BackupFile, settings config.OSSBackupSettings) error {
	if err := ValidateS3BackupSettings(settings); err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}

	object, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer object.Close()

	client, err := newS3Client(ctx, settings)
	if err != nil {
		return err
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(settings.Bucket),
		Key:         aws.String(s3ObjectKey(settings.Prefix, file.Name)),
		Body:        object,
		ContentType: aws.String("application/zip"),
	}
	if file.Size >= 0 {
		input.ContentLength = aws.Int64(file.Size)
	}
	if _, err := client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("上传 S3 备份失败: %w", err)
	}
	return nil
}

func (u *S3Uploader) Cleanup(ctx context.Context, settings config.OSSBackupSettings) error {
	if err := ValidateS3BackupSettings(settings); err != nil {
		return err
	}
	if !settings.Enabled || settings.Retention <= 0 {
		return nil
	}

	client, err := newS3Client(ctx, settings)
	if err != nil {
		return err
	}
	prefix := s3ObjectKey(settings.Prefix, backupFilePrefix)
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(settings.Bucket),
		Prefix: aws.String(prefix),
	})
	objects := make([]remoteBackupObject, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("列出 S3 备份失败: %w", err)
		}
		for _, object := range page.Contents {
			key := aws.ToString(object.Key)
			if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(strings.ToLower(key), ".zip") {
				continue
			}
			modifiedAt := time.Time{}
			if object.LastModified != nil {
				modifiedAt = *object.LastModified
			}
			objects = append(objects, remoteBackupObject{key: key, modifiedAt: modifiedAt})
		}
	}
	if len(objects) <= settings.Retention {
		return nil
	}
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].modifiedAt.Equal(objects[j].modifiedAt) {
			return objects[i].key > objects[j].key
		}
		return objects[i].modifiedAt.After(objects[j].modifiedAt)
	})
	for start := settings.Retention; start < len(objects); start += 1000 {
		end := start + 1000
		if end > len(objects) {
			end = len(objects)
		}
		identifiers := make([]types.ObjectIdentifier, 0, end-start)
		for _, object := range objects[start:end] {
			identifiers = append(identifiers, types.ObjectIdentifier{Key: aws.String(object.key)})
		}
		if _, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(settings.Bucket),
			Delete: &types.Delete{Objects: identifiers, Quiet: aws.Bool(true)},
		}); err != nil {
			return fmt.Errorf("删除 S3 旧备份失败: %w", err)
		}
	}
	return nil
}

type remoteBackupObject struct {
	key        string
	modifiedAt time.Time
}

func newS3Client(ctx context.Context, settings config.OSSBackupSettings) (*s3.Client, error) {
	region := strings.TrimSpace(settings.Region)
	if region == "" {
		region = "us-east-1"
	}
	awsSettings, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("加载 S3 客户端配置失败: %w", err)
	}
	awsSettings.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	if settings.AccessKey != "" || settings.SecretKey != "" {
		awsSettings.Credentials = credentials.NewStaticCredentialsProvider(settings.AccessKey, settings.SecretKey, settings.SessionToken)
	}
	return s3.NewFromConfig(awsSettings, func(options *s3.Options) {
		if endpoint := strings.TrimSpace(settings.Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		if normalizeAddressingStyle(settings.AddressingStyle) == "path" {
			options.UsePathStyle = true
		}
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	}), nil
}

func normalizeAddressingStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "path", "path-style", "path_style":
		return "path"
	case "virtual", "virtual-hosted", "virtual-hosted-style", "virtual_hosted", "virtual_hosted_style":
		return "virtual"
	default:
		return "auto"
	}
}

func s3ObjectKey(prefix, name string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return name
	}
	return pathpkg.Join(prefix, name)
}
