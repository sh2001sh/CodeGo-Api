package filex

import (
	"encoding/base64"
	"fmt"
	"image"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// GetImageConfig returns cached image metadata for a file source.
func GetImageConfig(c *gin.Context, source types.FileSource) (image.Config, string, error) {
	cachedData, err := LoadFileSource(c, source, "get_image_config")
	if err != nil {
		return image.Config{}, "", err
	}

	if cachedData.ImageConfig != nil {
		return *cachedData.ImageConfig, cachedData.ImageFormat, nil
	}

	base64Str, err := cachedData.GetBase64Data()
	if err != nil {
		return image.Config{}, "", fmt.Errorf("failed to get base64 data: %w", err)
	}
	decodedData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return image.Config{}, "", fmt.Errorf("failed to decode base64 for image config: %w", err)
	}

	config, format, err := decodeImageConfig(decodedData)
	if err != nil {
		return image.Config{}, "", err
	}

	cachedData.ImageConfig = &config
	cachedData.ImageFormat = format
	return config, format, nil
}

// GetBase64Data returns cached base64 payload and MIME type for a file source.
func GetBase64Data(c *gin.Context, source types.FileSource, reason ...string) (string, string, error) {
	cachedData, err := LoadFileSource(c, source, reason...)
	if err != nil {
		return "", "", err
	}
	base64Str, err := cachedData.GetBase64Data()
	if err != nil {
		return "", "", fmt.Errorf("failed to get base64 data: %w", err)
	}
	return base64Str, cachedData.MimeType, nil
}

// GetMimeType returns the MIME type for a file source.
func GetMimeType(c *gin.Context, source types.FileSource) (string, error) {
	if source.HasCache() {
		return source.GetCache().MimeType, nil
	}

	if urlSource, ok := source.(*types.URLSource); ok {
		mimeType, err := GetFileTypeFromURL(c, urlSource.URL, "get_mime_type")
		if err == nil && mimeType != "" && mimeType != "application/octet-stream" {
			return mimeType, nil
		}
	}

	cachedData, err := LoadFileSource(c, source, "get_mime_type")
	if err != nil {
		return "", err
	}
	return cachedData.MimeType, nil
}

// DetectFileType maps MIME types to domain file types.
func DetectFileType(mimeType string) types.FileType {
	if strings.HasPrefix(mimeType, "image/") {
		return types.FileTypeImage
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return types.FileTypeAudio
	}
	if strings.HasPrefix(mimeType, "video/") {
		return types.FileTypeVideo
	}
	return types.FileTypeFile
}

func guessMimeTypeFromURL(url string) string {
	cleanedURL := url
	if q := strings.Index(cleanedURL, "?"); q != -1 {
		cleanedURL = cleanedURL[:q]
	}

	if slash := strings.LastIndex(cleanedURL, "/"); slash != -1 && slash+1 < len(cleanedURL) {
		last := cleanedURL[slash+1:]
		if dot := strings.LastIndex(last, "."); dot != -1 && dot+1 < len(last) {
			ext := strings.ToLower(last[dot+1:])
			return GetMimeTypeByExtension(ext)
		}
	}

	return "application/octet-stream"
}
