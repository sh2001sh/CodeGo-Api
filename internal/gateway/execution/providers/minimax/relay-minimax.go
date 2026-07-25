package minimax

import (
	"fmt"

	channelconstant "github.com/sh2001sh/CodeGo-Api/constant"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeMiniMax]
	}

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return fmt.Sprintf("%s/anthropic/v1/messages", baseURL), nil
	default:
		switch info.RelayMode {
		case gatewaycontract.RelayModeChatCompletions:
			return fmt.Sprintf("%s/v1/text/chatcompletion_v2", baseURL), nil
		case gatewaycontract.RelayModeImagesGenerations:
			return fmt.Sprintf("%s/v1/image_generation", baseURL), nil
		case gatewaycontract.RelayModeAudioSpeech:
			return fmt.Sprintf("%s/v1/t2a_v2", baseURL), nil
		default:
			return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
		}
	}
}
