package runtime

import (
	"strings"

	"github.com/google/uuid"
)

// GetUUID returns a UUID string without hyphen separators.
func GetUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
