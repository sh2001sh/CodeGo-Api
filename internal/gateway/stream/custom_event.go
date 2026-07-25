package stream

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type stringWriter interface {
	io.Writer
	writeString(string) (int, error)
}

type stringWrapper struct {
	io.Writer
}

func (w stringWrapper) writeString(str string) (int, error) {
	return w.Writer.Write([]byte(str))
}

func checkWriter(writer io.Writer) stringWriter {
	if w, ok := writer.(stringWriter); ok {
		return w
	}
	return stringWrapper{writer}
}

var customEventContentType = []string{"text/event-stream"}
var customEventNoCache = []string{"no-cache"}

var customEventDataReplacer = strings.NewReplacer(
	"\n", "\n",
	"\r", "\\r",
)

// CustomEvent renders a single SSE payload chunk.
type CustomEvent struct {
	Event string
	Id    string
	Retry uint
	Data  interface{}

	Mutex sync.Mutex
}

func encodeCustomEvent(writer io.Writer, event CustomEvent) error {
	w := checkWriter(writer)
	return writeCustomEventData(w, event.Data)
}

func writeCustomEventData(w stringWriter, data interface{}) error {
	str, ok := data.(string)
	if !ok {
		return fmt.Errorf("custom event data must be string")
	}
	customEventDataReplacer.WriteString(w, fmt.Sprint(data))
	if strings.HasPrefix(str, "data") {
		_, _ = w.writeString("\n\n")
	}
	return nil
}

func (r CustomEvent) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	return encodeCustomEvent(w, r)
}

func (r CustomEvent) WriteContentType(w http.ResponseWriter) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	header := w.Header()
	header["Content-Type"] = customEventContentType
	if _, exist := header["Cache-Control"]; !exist {
		header["Cache-Control"] = customEventNoCache
	}
}
