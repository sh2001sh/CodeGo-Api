package main

import (
	gatewayexecution "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/bootstrap"
	workflowapp "github.com/sh2001sh/CodeGo-Api/internal/workflow/app"
)

func main() {
	bootstrap.SetRuntimeWiring(func() {
		gatewayexecutionapp.SetRelayRuntime(gatewayexecutionapp.RelayRuntime{
			Text:            gatewayexecution.TextHelper,
			Image:           gatewayexecution.ImageHelper,
			Audio:           gatewayexecution.AudioHelper,
			Rerank:          gatewayexecution.RerankHelper,
			Embedding:       gatewayexecution.EmbeddingHelper,
			Responses:       gatewayexecution.ResponsesHelper,
			Gemini:          gatewayexecution.GeminiHelper,
			GeminiEmbedding: gatewayexecution.GeminiEmbeddingHandler,
			Claude:          gatewayexecution.ClaudeHelper,
			Realtime:        gatewayexecution.WssHelper,
		})
		workflowapp.GetTaskAdaptorFunc = workflowapp.NewTaskPollingAdaptor
		workflowapp.GetTaskRelayAdaptorFunc = workflowapp.NewTaskRelayAdaptor
	})
	bootstrap.RunGatewayAPI()
}
