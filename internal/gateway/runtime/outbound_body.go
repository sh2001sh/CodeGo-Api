package runtime

import (
	"io"

	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

// NewOutboundJSONBody wraps a marshaled upstream payload into replayable body storage.
func NewOutboundJSONBody(data []byte) (body io.Reader, size int64, closer io.Closer, err error) {
	storage, err := platformhttpx.CreateBodyStorage(data)
	if err != nil {
		return nil, 0, nil, err
	}
	return platformhttpx.ReaderOnly(storage), storage.Size(), storage, nil
}
