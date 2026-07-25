package passkey

import (
	"fmt"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"strconv"
	"strings"

	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"

	webauthn "github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUser struct {
	user       *identityschema.User
	credential *identitydomain.PasskeyCredential
}

func NewWebAuthnUser(user *identityschema.User, credential *identitydomain.PasskeyCredential) *WebAuthnUser {
	return &WebAuthnUser{user: user, credential: credential}
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	if u == nil || u.user == nil {
		return nil
	}
	return []byte(strconv.Itoa(u.user.Id))
}

func (u *WebAuthnUser) WebAuthnName() string {
	if u == nil || u.user == nil {
		return ""
	}
	name := strings.TrimSpace(u.user.Username)
	if name == "" {
		return fmt.Sprintf("user-%d", u.user.Id)
	}
	return name
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u == nil || u.user == nil {
		return ""
	}
	display := strings.TrimSpace(u.user.DisplayName)
	if display != "" {
		return display
	}
	return u.WebAuthnName()
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	if u == nil || u.credential == nil {
		return nil
	}
	cred := ToWebAuthnCredential(u.credential)
	return []webauthn.Credential{cred}
}

func (u *WebAuthnUser) ModelUser() *identityschema.User {
	if u == nil {
		return nil
	}
	return u.user
}

func (u *WebAuthnUser) PasskeyCredential() *identitydomain.PasskeyCredential {
	if u == nil {
		return nil
	}
	return u.credential
}
