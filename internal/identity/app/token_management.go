package app

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"strings"

	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

var errInvalidToken = errors.New("invalid token")

func normalizeBearerTokenKey(tokenKey string) (string, []string) {
	key := strings.TrimSpace(tokenKey)
	if strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	key = strings.TrimPrefix(key, "sk-")
	parts := strings.Split(key, "-")
	return parts[0], parts
}

// ListUserTokens returns paginated tokens and the total count for a user.
func ListUserTokens(userID int, startIdx int, pageSize int) ([]*identityschema.Token, int64, error) {
	tokens, err := identitystore.ListUserTokens(userID, startIdx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := identitystore.CountUserTokens(userID)
	if err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}

// SearchUserTokens returns paginated token search results for a user.
func SearchUserTokens(userID int, keyword string, token string, startIdx int, pageSize int) ([]*identityschema.Token, int64, error) {
	return identitystore.SearchUserTokens(userID, keyword, token, startIdx, pageSize)
}

// GetUserToken returns a user-owned token by ID.
func GetUserToken(userID int, tokenID int) (*identityschema.Token, error) {
	return identitystore.LoadUserTokenByID(userID, tokenID)
}

// GetTokenByBearerKey returns a token by its bearer token value.
func GetTokenByBearerKey(tokenKey string) (*identityschema.Token, error) {
	key, _ := normalizeBearerTokenKey(tokenKey)
	return identitystore.LoadTokenByKey(key, false)
}

// ValidateUserBearerToken validates an API token and returns any suffix parts from the bearer key.
func ValidateUserBearerToken(tokenKey string) (*identityschema.Token, []string, error) {
	key, parts := normalizeBearerTokenKey(tokenKey)
	token, err := identitystore.ValidateUserToken(key)
	return token, parts, err
}

// CountTokensForUser returns the total number of tokens owned by the user.
func CountTokensForUser(userID int) (int64, error) {
	return identitystore.CountUserTokens(userID)
}

// InsertUserToken persists a newly created token.
func InsertUserToken(token *identityschema.Token) error {
	if token == nil {
		return errInvalidToken
	}
	return identitystore.CreateToken(token)
}

// DeleteUserToken deletes a user-owned token by ID.
func DeleteUserToken(userID int, tokenID int) error {
	return identitystore.DeleteUserToken(userID, tokenID)
}

// UpdateUserToken persists token field updates.
func UpdateUserToken(token *identityschema.Token) error {
	if token == nil {
		return errInvalidToken
	}
	return identitystore.UpdateToken(token)
}

// BatchDeleteUserTokens deletes a set of user-owned tokens.
func BatchDeleteUserTokens(userID int, ids []int) (int, error) {
	return identitystore.BatchDeleteUserTokens(userID, ids)
}

// GetUserTokenKeys returns a map of owned token IDs to full keys.
func GetUserTokenKeys(userID int, ids []int) (map[int]string, error) {
	tokens, err := identitystore.LoadUserTokenKeys(userID, ids)
	if err != nil {
		return nil, err
	}
	keys := make(map[int]string, len(tokens))
	for _, token := range tokens {
		tokenCopy := token
		keys[tokenCopy.Id] = tokenCopy.GetFullKey()
	}
	return keys, nil
}
