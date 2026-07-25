package passkey

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
)

func transportList(credential *identitydomain.PasskeyCredential) []protocol.AuthenticatorTransport {
	if credential == nil || strings.TrimSpace(credential.Transports) == "" {
		return nil
	}
	var transports []string
	if err := json.Unmarshal([]byte(credential.Transports), &transports); err != nil {
		return nil
	}
	result := make([]protocol.AuthenticatorTransport, 0, len(transports))
	for _, transport := range transports {
		result = append(result, protocol.AuthenticatorTransport(transport))
	}
	return result
}

func setTransports(credential *identitydomain.PasskeyCredential, list []protocol.AuthenticatorTransport) {
	if credential == nil {
		return
	}
	if len(list) == 0 {
		credential.Transports = ""
		return
	}
	stringList := make([]string, len(list))
	for i, transport := range list {
		stringList[i] = string(transport)
	}
	encoded, err := json.Marshal(stringList)
	if err != nil {
		return
	}
	credential.Transports = string(encoded)
}

func ToWebAuthnCredential(credential *identitydomain.PasskeyCredential) webauthn.Credential {
	if credential == nil {
		return webauthn.Credential{}
	}
	flags := webauthn.CredentialFlags{
		UserPresent:    credential.UserPresent,
		UserVerified:   credential.UserVerified,
		BackupEligible: credential.BackupEligible,
		BackupState:    credential.BackupState,
	}

	credID, _ := base64.StdEncoding.DecodeString(credential.CredentialID)
	pubKey, _ := base64.StdEncoding.DecodeString(credential.PublicKey)
	aaguid, _ := base64.StdEncoding.DecodeString(credential.AAGUID)

	return webauthn.Credential{
		ID:              credID,
		PublicKey:       pubKey,
		AttestationType: credential.AttestationType,
		Transport:       transportList(credential),
		Flags:           flags,
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    credential.SignCount,
			CloneWarning: credential.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(credential.Attachment),
		},
	}
}

func NewCredentialRecord(userID int, credential *webauthn.Credential) *identitydomain.PasskeyCredential {
	if credential == nil {
		return nil
	}
	passkeyCredential := &identitydomain.PasskeyCredential{
		UserID:          userID,
		CredentialID:    base64.StdEncoding.EncodeToString(credential.ID),
		PublicKey:       base64.StdEncoding.EncodeToString(credential.PublicKey),
		AttestationType: credential.AttestationType,
		AAGUID:          base64.StdEncoding.EncodeToString(credential.Authenticator.AAGUID),
		SignCount:       credential.Authenticator.SignCount,
		CloneWarning:    credential.Authenticator.CloneWarning,
		UserPresent:     credential.Flags.UserPresent,
		UserVerified:    credential.Flags.UserVerified,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		Attachment:      string(credential.Authenticator.Attachment),
	}
	setTransports(passkeyCredential, credential.Transport)
	return passkeyCredential
}
