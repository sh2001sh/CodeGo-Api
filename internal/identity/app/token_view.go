package app

import (
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
)

func BuildMaskedTokenResponse(token *identityschema.Token) *identityschema.Token {
	if token == nil {
		return nil
	}
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	return &maskedToken
}

func BuildMaskedTokenResponses(tokens []*identityschema.Token) []*identityschema.Token {
	maskedTokens := make([]*identityschema.Token, 0, len(tokens))
	for _, token := range tokens {
		maskedTokens = append(maskedTokens, BuildMaskedTokenResponse(token))
	}
	return maskedTokens
}
