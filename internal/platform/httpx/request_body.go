package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
)

const KeyRequestBody = "key_request_body"
const KeyBodyStorage = "key_body_storage"

var ErrRequestBodyTooLarge = errors.New("request body too large")

func IsRequestBodyTooLargeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRequestBodyTooLarge) {
		return true
	}
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func GetRequestBody(c *gin.Context) (io.Seeker, error) {
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(BodyStorage); ok {
			if _, err := bs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek body storage: %w", err)
			}
			return bs, nil
		}
	}

	if cached, exists := c.Get(KeyRequestBody); exists && cached != nil {
		if bodyBytes, ok := cached.([]byte); ok {
			bs, err := CreateBodyStorage(bodyBytes)
			if err != nil {
				return nil, err
			}
			c.Set(KeyBodyStorage, bs)
			return bs, nil
		}
	}

	maxMB := constant.MaxRequestBodyMB
	if maxMB <= 0 {
		maxMB = 128
	}
	maxBytes := int64(maxMB) << 20

	storage, err := CreateBodyStorageFromReader(c.Request.Body, c.Request.ContentLength, maxBytes)
	_ = c.Request.Body.Close()
	if err != nil {
		if IsRequestBodyTooLargeError(err) {
			return nil, fmt.Errorf("%w: request body exceeds %d MB", ErrRequestBodyTooLarge, maxMB)
		}
		return nil, err
	}

	c.Set(KeyBodyStorage, storage)
	return storage, nil
}

func GetBodyStorage(c *gin.Context) (BodyStorage, error) {
	seeker, err := GetRequestBody(c)
	if err != nil {
		return nil, err
	}
	storage, ok := seeker.(BodyStorage)
	if !ok {
		return nil, errors.New("unexpected body storage type")
	}
	return storage, nil
}

func CleanupBodyStorage(c *gin.Context) {
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(BodyStorage); ok {
			_ = bs.Close()
		}
		c.Set(KeyBodyStorage, nil)
	}
}

func UnmarshalBodyReusable(c *gin.Context, v any) error {
	storage, err := GetBodyStorage(c)
	if err != nil {
		return err
	}

	contentType := c.Request.Header.Get("Content-Type")
	if storage.IsDisk() && strings.HasPrefix(contentType, "application/json") {
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := json.NewDecoder(storage).Decode(v); err != nil {
			return err
		}
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return err
		}
		c.Request.Body = io.NopCloser(storage)
		return nil
	}

	requestBody, err := storage.Bytes()
	if err != nil {
		return err
	}

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		err = json.Unmarshal(requestBody, v)
	case strings.Contains(contentType, gin.MIMEPOSTForm):
		err = parseFormData(requestBody, v)
	case strings.Contains(contentType, gin.MIMEMultipartPOSTForm):
		err = parseMultipartFormData(c, requestBody, v)
	default:
	}
	if err != nil {
		return err
	}

	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(storage)
	return nil
}

func ParseMultipartFormReusable(c *gin.Context) (*multipart.Form, error) {
	storage, err := GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	requestBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}

	contentType := getOriginalMultipartContentType(c)
	boundary, err := parseBoundary(contentType)
	if err != nil {
		return nil, err
	}

	reader := multipart.NewReader(bytes.NewReader(requestBody), boundary)
	form, err := reader.ReadForm(multipartMemoryLimit())
	if err != nil {
		return nil, err
	}

	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(storage)
	return form, nil
}

func getOriginalMultipartContentType(c *gin.Context) string {
	if saved, ok := c.Get("_original_multipart_ct"); ok {
		return saved.(string)
	}
	contentType := c.Request.Header.Get("Content-Type")
	c.Set("_original_multipart_ct", contentType)
	return contentType
}

func processFormMap(formMap map[string]any, v any) error {
	jsonData, err := json.Marshal(formMap)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, v)
}

func parseFormData(data []byte, v any) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}

	formMap := make(map[string]any, len(values))
	for key, vals := range values {
		if len(vals) == 1 {
			formMap[key] = vals[0]
			continue
		}
		formMap[key] = vals
	}
	return processFormMap(formMap, v)
}

func parseMultipartFormData(c *gin.Context, data []byte, v any) error {
	boundary, err := parseBoundary(getOriginalMultipartContentType(c))
	if err != nil {
		if errors.Is(err, errBoundaryNotFound) {
			return json.Unmarshal(data, v)
		}
		return err
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)
	form, err := reader.ReadForm(multipartMemoryLimit())
	if err != nil {
		return err
	}
	defer form.RemoveAll()

	formMap := make(map[string]any, len(form.Value))
	for key, vals := range form.Value {
		if len(vals) == 1 {
			formMap[key] = vals[0]
			continue
		}
		formMap[key] = vals
	}
	return processFormMap(formMap, v)
}

var errBoundaryNotFound = errors.New("multipart boundary not found")

func parseBoundary(contentType string) (string, error) {
	if contentType == "" {
		return "", errBoundaryNotFound
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", err
	}
	boundary, ok := params["boundary"]
	if !ok || boundary == "" {
		return "", errBoundaryNotFound
	}
	return boundary, nil
}

func multipartMemoryLimit() int64 {
	limitMB := constant.MaxFileDownloadMB
	if limitMB <= 0 {
		limitMB = 32
	}
	return int64(limitMB) << 20
}
