package providers

func FetchOllamaModels(baseURL string, apiKey string) ([]OllamaModel, error) {
	return fetchOllamaModels(baseURL, apiKey)
}

func PullOllamaModel(baseURL string, apiKey string, modelName string) error {
	return pullOllamaModel(baseURL, apiKey, modelName)
}

func PullOllamaModelStream(baseURL string, apiKey string, modelName string, progressCallback func(OllamaPullResponse)) error {
	return pullOllamaModelStream(baseURL, apiKey, modelName, progressCallback)
}

func DeleteOllamaModel(baseURL string, apiKey string, modelName string) error {
	return deleteOllamaModel(baseURL, apiKey, modelName)
}

func FetchOllamaVersion(baseURL string, apiKey string) (string, error) {
	return fetchOllamaVersion(baseURL, apiKey)
}
