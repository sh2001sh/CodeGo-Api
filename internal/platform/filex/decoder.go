package filex

import (
	"bytes"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// GetFileTypeFromURL resolves a URL to a MIME type.
func GetFileTypeFromURL(c *gin.Context, url string, reason ...string) (string, error) {
	response, err := httpx.DoDownloadRequest(url, []string{"get_mime_type", strings.Join(reason, ", ")}...)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("fail to get file type from url: %s, error: %s", url, err.Error()))
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("failed to download file from %s, status code: %d", url, response.StatusCode))
		return "", fmt.Errorf("failed to download file, status code: %d", response.StatusCode)
	}

	if headerType := strings.TrimSpace(response.Header.Get("Content-Type")); headerType != "" {
		if i := strings.Index(headerType, ";"); i != -1 {
			headerType = headerType[:i]
		}
		if headerType != "application/octet-stream" {
			return headerType, nil
		}
	}

	if cd := response.Header.Get("Content-Disposition"); cd != "" {
		parts := strings.Split(cd, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "filename=") {
				name := strings.TrimSpace(strings.TrimPrefix(part, "filename="))
				if len(name) > 2 && name[0] == '"' && name[len(name)-1] == '"' {
					name = name[1 : len(name)-1]
				}
				if dot := strings.LastIndex(name, "."); dot != -1 && dot+1 < len(name) {
					ext := strings.ToLower(name[dot+1:])
					if ext != "" {
						mt := GetMimeTypeByExtension(ext)
						if mt != "application/octet-stream" {
							return mt, nil
						}
					}
				}
				break
			}
		}
	}

	cleanedURL := url
	if q := strings.Index(cleanedURL, "?"); q != -1 {
		cleanedURL = cleanedURL[:q]
	}
	if slash := strings.LastIndex(cleanedURL, "/"); slash != -1 && slash+1 < len(cleanedURL) {
		last := cleanedURL[slash+1:]
		if dot := strings.LastIndex(last, "."); dot != -1 && dot+1 < len(last) {
			ext := strings.ToLower(last[dot+1:])
			if ext != "" {
				mt := GetMimeTypeByExtension(ext)
				if mt != "application/octet-stream" {
					return mt, nil
				}
			}
		}
	}

	var readData []byte
	limits := []int{512, 8 * 1024, 24 * 1024, 64 * 1024}
	for _, limit := range limits {
		logger.LogDebug(c, fmt.Sprintf("Trying to read %d bytes to determine file type", limit))
		if len(readData) < limit {
			need := limit - len(readData)
			tmp := make([]byte, need)
			n, _ := io.ReadFull(response.Body, tmp)
			if n > 0 {
				readData = append(readData, tmp[:n]...)
			}
		}

		if len(readData) == 0 {
			continue
		}

		sniffed := http.DetectContentType(readData)
		if sniffed != "" && sniffed != "application/octet-stream" {
			return sniffed, nil
		}

		if heifMime := detectHEIF(readData); heifMime != "" {
			return heifMime, nil
		}

		if _, format, err := image.DecodeConfig(bytes.NewReader(readData)); err == nil {
			switch strings.ToLower(format) {
			case "jpeg", "jpg":
				return "image/jpeg", nil
			case "png":
				return "image/png", nil
			case "gif":
				return "image/gif", nil
			case "bmp":
				return "image/bmp", nil
			case "tiff":
				return "image/tiff", nil
			default:
				if format != "" {
					return "image/" + strings.ToLower(format), nil
				}
			}
		}
	}

	return "application/octet-stream", nil
}

// GetFileBase64FromURL loads a remote file and returns the legacy LocalFileData shape.
func GetFileBase64FromURL(c *gin.Context, url string, reason ...string) (*types.LocalFileData, error) {
	source := types.NewURLFileSource(url)
	cachedData, err := LoadFileSource(c, source, reason...)
	if err != nil {
		return nil, err
	}

	base64Data, err := cachedData.GetBase64Data()
	if err != nil {
		return nil, err
	}
	return &types.LocalFileData{
		Base64Data: base64Data,
		MimeType:   cachedData.MimeType,
		Size:       cachedData.Size,
		Url:        url,
	}, nil
}

// GetMimeTypeByExtension returns the MIME type for a known file extension.
func GetMimeTypeByExtension(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case "txt", "md", "markdown", "csv", "json", "xml", "html", "htm":
		return "text/plain"
	case "jpg", "jpeg", "jfif":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
	case "mp3":
		return "audio/mp3"
	case "wav":
		return "audio/wav"
	case "mpeg":
		return "audio/mpeg"
	case "mp4":
		return "video/mp4"
	case "wmv":
		return "video/wmv"
	case "flv":
		return "video/flv"
	case "mov":
		return "video/mov"
	case "mpg":
		return "video/mpg"
	case "avi":
		return "video/avi"
	case "mpegps":
		return "video/mpegps"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
