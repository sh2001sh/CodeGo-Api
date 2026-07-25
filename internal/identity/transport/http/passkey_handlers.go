package http

import (
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	passkeyapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app/passkey"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/sessionstate"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
)

func PasskeyRegisterBegin(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}
	if err := requirePasskeyRegistrationVerification(c, user.Id); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}

	credential, err := identityapp.LoadOptionalPasskeyCredential(user.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	waUser := passkeyapp.NewWebAuthnUser(user, credential)
	var options []webauthnlib.RegistrationOption
	if credential != nil {
		descriptor := passkeyapp.ToWebAuthnCredential(credential).Descriptor()
		options = append(options, webauthnlib.WithExclusions([]protocol.CredentialDescriptor{descriptor}))
	}

	creation, sessionData, err := wa.BeginRegistration(waUser, options...)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := passkeyapp.SaveSessionData(c, passkeyapp.RegistrationSessionKey, sessionData); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	httpapi.ApiSuccess(c, gin.H{"options": creation})
}

func PasskeyRegisterFinish(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}
	if err := requirePasskeyRegistrationVerification(c, user.Id); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	credentialRecord, err := identityapp.LoadOptionalPasskeyCredential(user.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	sessionData, err := passkeyapp.PopSessionData(c, passkeyapp.RegistrationSessionKey)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	waUser := passkeyapp.NewWebAuthnUser(user, credentialRecord)
	credential, err := wa.FinishRegistration(waUser, *sessionData, c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := identityapp.StoreRegisteredPasskeyCredential(user.Id, credential); err != nil {
		handlePasskeyAppError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Passkey 注册成功"})
}

func PasskeyDelete(c *gin.Context) {
	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}
	if err := requirePasskeyDeleteVerification(c, user.Id); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}
	if err := identityapp.DeletePasskeyBinding(user.Id); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Passkey 已解绑"})
}

func PasskeyStatus(c *gin.Context) {
	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}

	status, err := identityapp.LoadPasskeyStatus(user.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, status)
}

func PasskeyLoginBegin(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	assertion, sessionData, err := wa.BeginDiscoverableLogin()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := passkeyapp.SaveSessionData(c, passkeyapp.LoginSessionKey, sessionData); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	httpapi.ApiSuccess(c, gin.H{"options": assertion})
}

func PasskeyLoginFinish(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	sessionData, err := passkeyapp.PopSessionData(c, passkeyapp.LoginSessionKey)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	handler := func(rawID, userHandle []byte) (webauthnlib.User, error) {
		user, credential, err := identityapp.ResolvePasskeyLoginUser(rawID, userHandle)
		if err != nil {
			return nil, err
		}
		return passkeyapp.NewWebAuthnUser(user, credential), nil
	}

	waUser, credential, err := wa.FinishPasskeyLogin(handler, *sessionData, c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	userWrapper, ok := waUser.(*passkeyapp.WebAuthnUser)
	if !ok {
		httpapi.ApiErrorMsg(c, identityapp.ErrPasskeyLoginState.Error())
		return
	}
	modelUser := userWrapper.ModelUser()
	if modelUser == nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrPasskeyLoginState.Error())
		return
	}

	if err := identityapp.StoreValidatedLoginCredential(modelUser.Id, credential); err != nil {
		handlePasskeyAppError(c, err)
		return
	}
	if err := establishAuthenticatedSession(c, sessions.Default(c), identityapp.BuildAuthenticatedSessionUser(modelUser)); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
	}
}

func AdminResetPasskey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrPasskeyInvalidUserID.Error())
		return
	}
	if err := identityapp.ResetPasskeyBinding(id); err != nil {
		handlePasskeyAppError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Passkey 已重置"})
}

func PasskeyVerifyBegin(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}

	credential, err := identityapp.LoadOptionalPasskeyCredential(user.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if credential == nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrPasskeyNotBound.Error())
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	waUser := passkeyapp.NewWebAuthnUser(user, credential)
	assertion, sessionData, err := wa.BeginLogin(waUser)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := passkeyapp.SaveSessionData(c, passkeyapp.VerifySessionKey, sessionData); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	httpapi.ApiSuccess(c, gin.H{"options": assertion})
}

func PasskeyVerifyFinish(c *gin.Context) {
	if !isPasskeyEnabled() {
		httpapi.ApiErrorMsg(c, "管理员未启用 Passkey 登录")
		return
	}

	user, err := currentPasskeyUser(c)
	if err != nil {
		handlePasskeySessionUserError(c, err)
		return
	}

	wa, err := passkeyapp.BuildWebAuthn(c.Request)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	credential, err := identityapp.LoadOptionalPasskeyCredential(user.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if credential == nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrPasskeyNotBound.Error())
		return
	}

	sessionData, err := passkeyapp.PopSessionData(c, passkeyapp.VerifySessionKey)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	waUser := passkeyapp.NewWebAuthnUser(user, credential)
	if _, err := wa.FinishLogin(waUser, *sessionData, c.Request); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := identityapp.MarkPasskeyCredentialUsed(credential); err != nil {
		handlePasskeyAppError(c, err)
		return
	}
	if err := sessionstate.MarkPasskeyReady(c); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Passkey 验证成功"})
}

func currentPasskeyUser(c *gin.Context) (*identityschema.User, error) {
	if userID := c.GetInt("id"); userID > 0 {
		return identityapp.LoadActiveUser(userID)
	}

	session := sessions.Default(c)
	idRaw := session.Get("id")
	if idRaw == nil {
		return nil, identityapp.ErrPasskeyNotLoggedIn
	}
	id, ok := idRaw.(int)
	if !ok {
		return nil, identityapp.ErrPasskeyInvalidSession
	}
	return identityapp.LoadActiveUser(id)
}

func requirePasskeyRegistrationVerification(c *gin.Context, userID int) error {
	twoFA, err := identityapp.LoadEnabledTwoFA(userID)
	if err != nil {
		return err
	}
	if twoFA == nil {
		return nil
	}
	return sessionstate.RequireSecureVerificationMethod(c, sessionstate.SecureVerificationMethod2FA)
}

func requirePasskeyDeleteVerification(c *gin.Context, userID int) error {
	twoFA, err := identityapp.LoadEnabledTwoFA(userID)
	if err != nil {
		return err
	}
	if twoFA != nil {
		return sessionstate.RequireSecureVerificationMethod(c, sessionstate.SecureVerificationMethod2FA)
	}

	credential, err := identityapp.LoadOptionalPasskeyCredential(userID)
	if err != nil {
		return err
	}
	if credential == nil {
		return identityapp.ErrPasskeyNotBound
	}
	return sessionstate.RequireSecureVerificationMethod(c, sessionstate.SecureVerificationMethodPasskey)
}

func isPasskeyEnabled() bool {
	return platformstore.GetPasskeySettings().Enabled
}

func handlePasskeySessionUserError(c *gin.Context, err error) {
	switch err {
	case identityapp.ErrPasskeyNotLoggedIn, identityapp.ErrPasskeyInvalidSession, identityapp.ErrPasskeyUserDisabled:
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
	default:
		httpapi.ApiError(c, err)
	}
}

func handlePasskeyAppError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case err == identityapp.ErrPasskeyNotBound,
		err == identityapp.ErrPasskeyInvalidUserID,
		err == identityapp.ErrPasskeyCredentialCreate,
		err == identityapp.ErrPasskeyCredentialUpdate,
		err == identityapp.ErrPasskeyLoginState,
		err == sessionstate.ErrSecureVerificationRequired,
		err == sessionstate.ErrSecureVerificationMethodMismatch:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
