package types

// IsRemoteProviderError reports whether an error originated from a channel or its provider.
func IsRemoteProviderError(err *APIError) bool {
	if err == nil {
		return false
	}
	return IsChannelError(err) || err.errorType == ErrorTypeOpenAIError || err.errorType == ErrorTypeClaudeError
}
