package filex

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func getContextCacheKey(url string) string {
	return fmt.Sprintf("file_cache_%s", platformsecurity.GenerateHMAC(url))
}

func getBase64ContextCacheKey(data string, mimeType string) string {
	keyMaterial := fmt.Sprintf("%d:%s:", len(data), mimeType)
	if len(data) > 128 {
		keyMaterial += data[:128]
	} else {
		keyMaterial += data
	}
	return fmt.Sprintf("b64_cache_%s", platformsecurity.GenerateHMAC(keyMaterial))
}

// LoadFileSource loads a file source into request-scoped cache.
func LoadFileSource(c *gin.Context, source types.FileSource, reason ...string) (*types.CachedFileData, error) {
	if source == nil {
		return nil, fmt.Errorf("file source is nil")
	}

	if platformconfig.DebugEnabled {
		logger.LogDebug(c, fmt.Sprintf("LoadFileSource starting for: %s", source.GetIdentifier()))
	}

	if source.HasCache() {
		if c != nil {
			registerSourceForCleanup(c, source)
		}
		return source.GetCache(), nil
	}

	source.Mu().Lock()
	defer source.Mu().Unlock()

	if source.HasCache() {
		if c != nil {
			registerSourceForCleanup(c, source)
		}
		return source.GetCache(), nil
	}

	var cachedData *types.CachedFileData
	var contextKey string
	var err error

	switch s := source.(type) {
	case *types.URLSource:
		if c != nil {
			contextKey = getContextCacheKey(s.URL)
			if cached, exists := c.Get(contextKey); exists {
				data := cached.(*types.CachedFileData)
				source.SetCache(data)
				registerSourceForCleanup(c, source)
				return data, nil
			}
		}
		cachedData, err = loadFromURL(c, s.URL, reason...)
	case *types.Base64Source:
		if c != nil {
			contextKey = getBase64ContextCacheKey(s.Base64Data, s.MimeType)
			if cached, exists := c.Get(contextKey); exists {
				data := cached.(*types.CachedFileData)
				source.SetCache(data)
				registerSourceForCleanup(c, source)
				return data, nil
			}
		}
		cachedData, err = loadFromBase64(s.Base64Data, s.MimeType)
	default:
		return nil, fmt.Errorf("unsupported file source type: %T", source)
	}

	if err != nil {
		return nil, err
	}

	source.SetCache(cachedData)
	if contextKey != "" && c != nil {
		c.Set(contextKey, cachedData)
	}
	if c != nil {
		registerSourceForCleanup(c, source)
	}
	return cachedData, nil
}

func registerSourceForCleanup(c *gin.Context, source types.FileSource) {
	if source.IsRegistered() {
		return
	}

	key := string(constant.ContextKeyFileSourcesToCleanup)
	var sources []types.FileSource
	if existing, exists := c.Get(key); exists {
		sources = existing.([]types.FileSource)
	}
	sources = append(sources, source)
	c.Set(key, sources)
	source.SetRegistered(true)
}

// CleanupFileSources releases file caches registered on the current request.
func CleanupFileSources(c *gin.Context) {
	key := string(constant.ContextKeyFileSourcesToCleanup)
	if sources, exists := c.Get(key); exists {
		for _, source := range sources.([]types.FileSource) {
			if cache := source.GetCache(); cache != nil {
				cache.Close()
			}
		}
		c.Set(key, nil)
	}
}

func loadFromURL(c *gin.Context, url string, reason ...string) (*types.CachedFileData, error) {
	maxFileSize := constant.MaxFileDownloadMB * 1024 * 1024

	if platformconfig.DebugEnabled {
		logger.LogDebug(c, "loadFromURL: initiating download")
	}
	resp, err := httpx.DoDownloadRequest(url, reason...)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	if platformconfig.DebugEnabled {
		logger.LogDebug(c, "loadFromURL: reading response body")
	}
	fileBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxFileSize+1)))
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	if len(fileBytes) > maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size: %dMB", constant.MaxFileDownloadMB)
	}

	base64Data := base64.StdEncoding.EncodeToString(fileBytes)
	mimeType := smartDetectMimeType(resp, url, fileBytes)

	base64Size := int64(len(base64Data))
	var cachedData *types.CachedFileData
	if shouldUseDiskCache(base64Size) {
		diskPath, err := writeToDiskCache(base64Data)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("Failed to write to disk cache, falling back to memory: %v", err))
			cachedData = types.NewMemoryCachedData(base64Data, mimeType, int64(len(fileBytes)))
		} else {
			cachedData = types.NewDiskCachedData(diskPath, mimeType, int64(len(fileBytes)))
			cachedData.DiskSize = base64Size
			cachedData.OnClose = func(size int64) {
				platformcache.DecrementDiskFiles(size)
			}
			platformcache.IncrementDiskFiles(base64Size)
			if platformconfig.DebugEnabled {
				logger.LogDebug(c, fmt.Sprintf("File cached to disk: %s, size: %d bytes", diskPath, base64Size))
			}
		}
	} else {
		cachedData = types.NewMemoryCachedData(base64Data, mimeType, int64(len(fileBytes)))
	}

	if strings.HasPrefix(mimeType, "image/") {
		if platformconfig.DebugEnabled {
			logger.LogDebug(c, "loadFromURL: decoding image config")
		}
		config, format, err := decodeImageConfig(fileBytes)
		if err == nil {
			cachedData.ImageConfig = &config
			cachedData.ImageFormat = format
			if mimeType == "application/octet-stream" || mimeType == "" {
				cachedData.MimeType = "image/" + format
			}
		}
	}

	return cachedData, nil
}

func shouldUseDiskCache(dataSize int64) bool {
	return platformcache.ShouldUseDiskCache(dataSize)
}

func writeToDiskCache(base64Data string) (string, error) {
	return platformcache.WriteDiskCacheFileString(platformcache.DiskCacheTypeFile, base64Data)
}

func smartDetectMimeType(resp *http.Response, url string, fileBytes []byte) string {
	mimeType := resp.Header.Get("Content-Type")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType != "" && mimeType != "application/octet-stream" {
		return mimeType
	}

	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
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
							return mt
						}
					}
				}
				break
			}
		}
	}

	mt := guessMimeTypeFromURL(url)
	if mt != "application/octet-stream" {
		return mt
	}

	if len(fileBytes) > 0 {
		sniffed := http.DetectContentType(fileBytes)
		if sniffed != "" && sniffed != "application/octet-stream" {
			if idx := strings.Index(sniffed, ";"); idx != -1 {
				sniffed = strings.TrimSpace(sniffed[:idx])
			}
			return sniffed
		}

		if heifMime := detectHEIF(fileBytes); heifMime != "" {
			return heifMime
		}
	}

	if len(fileBytes) > 0 {
		if _, format, err := decodeImageConfig(fileBytes); err == nil && format != "" {
			return "image/" + strings.ToLower(format)
		}
	}

	return "application/octet-stream"
}

func loadFromBase64(base64String string, providedMimeType string) (*types.CachedFileData, error) {
	var mimeType string
	var cleanBase64 string

	if strings.HasPrefix(base64String, "data:") {
		idx := strings.Index(base64String, ",")
		if idx != -1 {
			header := base64String[:idx]
			cleanBase64 = base64String[idx+1:]

			if strings.Contains(header, ":") && strings.Contains(header, ";") {
				mimeStart := strings.Index(header, ":") + 1
				mimeEnd := strings.Index(header, ";")
				if mimeStart < mimeEnd {
					mimeType = header[mimeStart:mimeEnd]
				}
			}
		} else {
			cleanBase64 = base64String
		}
	} else {
		cleanBase64 = base64String
	}

	if providedMimeType != "" {
		mimeType = providedMimeType
	}

	decodedData, err := base64.StdEncoding.DecodeString(cleanBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	base64Size := int64(len(cleanBase64))
	var cachedData *types.CachedFileData
	if shouldUseDiskCache(base64Size) {
		diskPath, err := writeToDiskCache(cleanBase64)
		if err != nil {
			cachedData = types.NewMemoryCachedData(cleanBase64, mimeType, int64(len(decodedData)))
		} else {
			cachedData = types.NewDiskCachedData(diskPath, mimeType, int64(len(decodedData)))
			cachedData.DiskSize = base64Size
			cachedData.OnClose = func(size int64) {
				platformcache.DecrementDiskFiles(size)
			}
			platformcache.IncrementDiskFiles(base64Size)
		}
	} else {
		cachedData = types.NewMemoryCachedData(cleanBase64, mimeType, int64(len(decodedData)))
	}

	if mimeType == "" || strings.HasPrefix(mimeType, "image/") {
		config, format, err := decodeImageConfig(decodedData)
		if err == nil {
			cachedData.ImageConfig = &config
			cachedData.ImageFormat = format
			if mimeType == "" {
				cachedData.MimeType = "image/" + format
			}
		}
	}

	return cachedData, nil
}
