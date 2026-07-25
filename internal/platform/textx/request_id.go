package textx

import "fmt"

// MessageWithRequestID appends the request identifier to a user-facing message.
func MessageWithRequestID(message string, id string) string {
	return fmt.Sprintf("%s (request id: %s)", message, id)
}
