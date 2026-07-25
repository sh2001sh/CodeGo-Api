package filex

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

func DecodeBase64FileData(base64String string) (string, string, error) {
	mimeType := ""
	idx := strings.Index(base64String, ",")
	if idx == -1 {
		format, cleanBase64, err := decodeBase64ImageData(base64String)
		return "image/" + format, cleanBase64, err
	}

	mimeType = base64String[:idx]
	base64String = base64String[idx+1:]
	idx = strings.Index(mimeType, ";")
	if idx == -1 {
		format, cleanBase64, err := decodeBase64ImageData(base64String)
		return "image/" + format, cleanBase64, err
	}

	mimeType = mimeType[:idx]
	idx = strings.Index(mimeType, ":")
	if idx == -1 {
		format, cleanBase64, err := decodeBase64ImageData(base64String)
		return "image/" + format, cleanBase64, err
	}

	return mimeType[idx+1:], base64String, nil
}

func GetImageFromURL(originURL string) (mimeType string, data string, err error) {
	resp, err := httpx.DoDownloadRequest(originURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" && !strings.HasPrefix(contentType, "image/") {
		return "", "", fmt.Errorf("invalid content type: %s, required image/*", contentType)
	}

	maxImageSize := int64(constant.MaxFileDownloadMB * 1024 * 1024)
	if resp.ContentLength > maxImageSize {
		return "", "", fmt.Errorf("image size %d exceeds maximum allowed size of %d bytes", resp.ContentLength, maxImageSize)
	}

	limitReader := io.LimitReader(resp.Body, maxImageSize)
	buffer := &bytes.Buffer{}
	written, err := io.Copy(buffer, limitReader)
	if err != nil {
		return "", "", fmt.Errorf("failed to read image data: %w", err)
	}
	if written >= maxImageSize {
		return "", "", fmt.Errorf("image size exceeds maximum allowed size of %d bytes", maxImageSize)
	}

	data = base64.StdEncoding.EncodeToString(buffer.Bytes())
	mimeType = contentType
	if mimeType == "application/octet-stream" {
		format, _, err := decodeBase64ImageData(data)
		if err != nil {
			return "", "", err
		}
		mimeType = "image/" + format
	}
	return mimeType, data, nil
}

func decodeBase64ImageData(base64String string) (string, string, error) {
	if idx := strings.Index(base64String, ","); idx != -1 {
		base64String = base64String[idx+1:]
	}
	if len(base64String) == 0 {
		return "", "", fmt.Errorf("base64 string is empty")
	}

	decodedData, err := base64.StdEncoding.DecodeString(base64String)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64 string: %w", err)
	}

	_, format, err := decodeImageConfig(decodedData)
	if err != nil {
		return "", "", err
	}
	return format, base64String, nil
}
