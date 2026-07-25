package tokenx

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func parseAudio(audioBase64 string, format string) (duration float64, err error) {
	audioData, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return 0, fmt.Errorf("base64 decode error: %v", err)
	}

	var samplesCount int
	var sampleRate int
	switch format {
	case "pcm16":
		samplesCount = len(audioData) / 2
		sampleRate = 24000
	case "g711_ulaw", "g711_alaw":
		samplesCount = len(audioData)
		sampleRate = 8000
	default:
		samplesCount = len(audioData)
		sampleRate = 8000
	}

	duration = float64(samplesCount) / float64(sampleRate)
	return duration, nil
}

// DecodeBase64AudioData validates and normalizes an inline audio payload.
func DecodeBase64AudioData(audioBase64 string) (string, error) {
	idx := strings.Index(audioBase64, ",")
	if idx != -1 {
		audioBase64 = audioBase64[idx+1:]
	}

	if _, err := base64.StdEncoding.DecodeString(audioBase64); err != nil {
		return "", fmt.Errorf("base64 decode error: %v", err)
	}
	return audioBase64, nil
}
