package backup

import (
	"context"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"strings"

	"github.com/allbot/allbot/core/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	region := strings.TrimSpace(settings.Region)
	if region == "" {
		region = "us-east-1"
	}
	awsSettings, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return fmt.Errorf("加载 S3 客户端配置失败: %w", err)
	}
	awsSettings.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	if settings.AccessKey != "" || settings.SecretKey != "" {
		awsSettings.Credentials = credentials.NewStaticCredentialsProvider(settings.AccessKey, settings.SecretKey, settings.SessionToken)
	}

	client := s3.NewFromConfig(awsSettings, func(options *s3.Options) {
		if endpoint := strings.TrimSpace(settings.Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		if normalizeAddressingStyle(settings.AddressingStyle) == "path" {
			options.UsePathStyle = true
		}
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

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
