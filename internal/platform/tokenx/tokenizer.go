package tokenx

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"sync"

	"github.com/tiktoken-go/tokenizer"
	"github.com/tiktoken-go/tokenizer/codec"
)

var defaultTokenEncoder tokenizer.Codec

var tokenEncoderMap = make(map[string]tokenizer.Codec)

var tokenEncoderMutex sync.RWMutex

// InitTokenEncoders initializes the shared tokenizer cache.
func InitTokenEncoders() {
	platformobservability.SysLog("initializing token encoders")
	defaultTokenEncoder = codec.NewCl100kBase()
	platformobservability.SysLog("token encoders initialized")
}

func getTokenEncoder(model string) tokenizer.Codec {
	tokenEncoderMutex.RLock()
	if encoder, exists := tokenEncoderMap[model]; exists {
		tokenEncoderMutex.RUnlock()
		return encoder
	}
	tokenEncoderMutex.RUnlock()

	tokenEncoderMutex.Lock()
	defer tokenEncoderMutex.Unlock()

	if encoder, exists := tokenEncoderMap[model]; exists {
		return encoder
	}

	modelCodec, err := tokenizer.ForModel(tokenizer.Model(model))
	if err != nil {
		tokenEncoderMap[model] = defaultTokenEncoder
		return defaultTokenEncoder
	}

	tokenEncoderMap[model] = modelCodec
	return modelCodec
}

func getTokenNum(tokenEncoder tokenizer.Codec, text string) int {
	if text == "" {
		return 0
	}
	tkm, _ := tokenEncoder.Count(text)
	return tkm
}
