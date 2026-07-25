package httpx

import (
	"bytes"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
)

// BodyStorage stores request bodies in memory or on disk while supporting replay.
type BodyStorage interface {
	io.ReadSeeker
	io.Closer
	Bytes() ([]byte, error)
	Size() int64
	IsDisk() bool
}

var ErrStorageClosed = fmt.Errorf("body storage is closed")

type memoryStorage struct {
	data   []byte
	reader *bytes.Reader
	size   int64
	closed int32
	mu     sync.Mutex
}

func newMemoryStorage(data []byte) *memoryStorage {
	size := int64(len(data))
	platformcache.IncrementMemoryBuffers(size)
	return &memoryStorage{
		data:   data,
		reader: bytes.NewReader(data),
		size:   size,
	}
}

func (m *memoryStorage) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Read(p)
}

func (m *memoryStorage) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Seek(offset, whence)
}

func (m *memoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		platformcache.DecrementMemoryBuffers(m.size)
	}
	return nil
}

func (m *memoryStorage) Bytes() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return nil, ErrStorageClosed
	}
	return m.data, nil
}

func (m *memoryStorage) Size() int64 {
	return m.size
}

func (m *memoryStorage) IsDisk() bool {
	return false
}

type diskStorage struct {
	file     *os.File
	filePath string
	size     int64
	closed   int32
	mu       sync.Mutex
}

func newDiskStorage(data []byte) (*diskStorage, error) {
	filePath, file, err := platformcache.CreateDiskCacheFile(platformcache.DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	n, err := file.Write(data)
	if err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	size := int64(n)
	platformcache.IncrementDiskFiles(size)

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     size,
	}, nil
}

func newDiskStorageFromReader(reader io.Reader, maxBytes int64) (*diskStorage, error) {
	filePath, file, err := platformcache.CreateDiskCacheFile(platformcache.DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	written, err := io.Copy(file, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if written > maxBytes {
		file.Close()
		os.Remove(filePath)
		return nil, ErrRequestBodyTooLarge
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	platformcache.IncrementDiskFiles(written)

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     written,
	}, nil
}

func (d *diskStorage) Read(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Read(p)
}

func (d *diskStorage) Seek(offset int64, whence int) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Seek(offset, whence)
}

func (d *diskStorage) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		d.file.Close()
		os.Remove(d.filePath)
		platformcache.DecrementDiskFiles(d.size)
	}
	return nil
}

func (d *diskStorage) Bytes() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return nil, ErrStorageClosed
	}

	currentPos, err := d.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, d.size)
	if _, err := io.ReadFull(d.file, data); err != nil {
		return nil, err
	}
	if _, err := d.file.Seek(currentPos, io.SeekStart); err != nil {
		return nil, err
	}
	return data, nil
}

func (d *diskStorage) Size() int64 {
	return d.size
}

func (d *diskStorage) IsDisk() bool {
	return true
}

func CreateBodyStorage(data []byte) (BodyStorage, error) {
	size := int64(len(data))
	threshold := platformcache.GetDiskCacheThresholdBytes()
	if platformcache.IsDiskCacheEnabled() &&
		size >= threshold &&
		platformcache.IsDiskCacheAvailable(size) {
		storage, err := newDiskStorage(data)
		if err != nil {
			platformobservability.SysError(fmt.Sprintf("failed to create disk storage, falling back to memory: %v", err))
			return newMemoryStorage(data), nil
		}
		return storage, nil
	}
	return newMemoryStorage(data), nil
}

func CreateBodyStorageFromReader(reader io.Reader, contentLength int64, maxBytes int64) (BodyStorage, error) {
	threshold := platformcache.GetDiskCacheThresholdBytes()
	if platformcache.IsDiskCacheEnabled() &&
		contentLength > 0 &&
		contentLength >= threshold &&
		platformcache.IsDiskCacheAvailable(contentLength) {
		storage, err := newDiskStorageFromReader(reader, maxBytes)
		if err != nil {
			if IsRequestBodyTooLargeError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("disk storage creation failed: %w", err)
		}
		platformcache.IncrementDiskCacheHits()
		return storage, nil
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrRequestBodyTooLarge
	}

	storage, err := CreateBodyStorage(data)
	if err != nil {
		return nil, err
	}
	if storage.IsDisk() {
		platformcache.IncrementDiskCacheHits()
	} else {
		platformcache.IncrementMemoryCacheHits()
	}
	return storage, nil
}

// ReaderOnly hides io.Closer so request constructors do not close the shared storage.
func ReaderOnly(r io.Reader) io.Reader {
	return struct{ io.Reader }{r}
}

func CleanupOldCacheFiles() {
	_ = platformcache.CleanupOldDiskCacheFiles(5 * time.Minute)
}
