package app

import (
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayproviders "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"strings"
)

// OllamaModelRequest describes pull/delete operations for an Ollama-backed channel.
type OllamaModelRequest struct {
	ChannelID int    `json:"channel_id"`
	ModelName string `json:"model_name"`
}

// OllamaStreamEvent represents a stream update emitted during model pull.
type OllamaStreamEvent struct {
	Data []byte
}

var (
	pullOllamaModel       = gatewayproviders.PullOllamaModel
	pullOllamaModelStream = gatewayproviders.PullOllamaModelStream
	deleteOllamaModel     = gatewayproviders.DeleteOllamaModel
	fetchOllamaVersion    = gatewayproviders.FetchOllamaVersion
)

func getOllamaChannel(channelID int) (*gatewayschema.Channel, error) {
	channel, err := gatewaystore.LoadChannelByID(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel.Type != constant.ChannelTypeOllama {
		return nil, fmt.Errorf("This operation is only supported for Ollama channels")
	}
	return channel, nil
}

func resolveOllamaBaseURL(channel *gatewayschema.Channel) string {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}
	return baseURL
}

func resolveOllamaKey(channel *gatewayschema.Channel) string {
	return strings.Split(channel.Key, "\n")[0]
}

// PullOllamaModel pulls one model for an Ollama-backed channel.
func PullOllamaModel(req OllamaModelRequest) error {
	channel, err := getOllamaChannel(req.ChannelID)
	if err != nil {
		return err
	}
	return pullOllamaModel(resolveOllamaBaseURL(channel), resolveOllamaKey(channel), req.ModelName)
}

// StreamPullOllamaModel streams pull progress for one model.
func StreamPullOllamaModel(req OllamaModelRequest, onProgress func(OllamaStreamEvent)) error {
	channel, err := getOllamaChannel(req.ChannelID)
	if err != nil {
		return err
	}

	progressCallback := func(progress gatewayproviders.OllamaPullResponse) {
		if onProgress == nil {
			return
		}
		data, _ := platformencoding.Marshal(progress)
		onProgress(OllamaStreamEvent{Data: data})
	}

	return pullOllamaModelStream(
		resolveOllamaBaseURL(channel),
		resolveOllamaKey(channel),
		req.ModelName,
		progressCallback,
	)
}

// DeleteOllamaModel deletes one model from an Ollama-backed channel.
func DeleteOllamaModel(req OllamaModelRequest) error {
	channel, err := getOllamaChannel(req.ChannelID)
	if err != nil {
		return err
	}
	return deleteOllamaModel(resolveOllamaBaseURL(channel), resolveOllamaKey(channel), req.ModelName)
}

// GetOllamaVersion returns the remote Ollama service version for a channel.
func GetOllamaVersion(channelID int) (string, error) {
	channel, err := getOllamaChannel(channelID)
	if err != nil {
		return "", err
	}
	return fetchOllamaVersion(resolveOllamaBaseURL(channel), resolveOllamaKey(channel))
}
