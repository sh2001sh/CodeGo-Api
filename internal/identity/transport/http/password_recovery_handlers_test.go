package http

import (
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"net/http"
	"testing"
)

func TestSendEmailVerificationRegistersCodeForAvailableEmail(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	restore := stubIdentitySendEmail(func(subject string, receiver string, content string) error {
		if receiver != "verify@example.com" {
			t.Fatalf("unexpected receiver: %s", receiver)
		}
		return nil
	})
	defer restore()

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/verification?email=verify@example.com", nil, 0)
	SendEmailVerification(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected verification success, got %#v", response)
	}
}

func TestSendEmailVerificationRejectsTakenEmail(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "taken-email-user",
		Password:    "password123",
		DisplayName: "Taken Email User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		Email:       "taken@example.com",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/verification?email=taken@example.com", nil, 0)
	SendEmailVerification(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected verification failure, got %#v", response)
	}
	if response.Message != "邮箱地址已被占用" {
		t.Fatalf("expected taken email error, got %q", response.Message)
	}
}

func TestSendPasswordResetEmailReturnsSuccessWhenEmailMissing(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	restore := stubIdentitySendEmail(func(subject string, receiver string, content string) error {
		t.Fatal("did not expect email to be sent")
		return nil
	})
	defer restore()

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/reset_password?email=missing@example.com", nil, 0)
	SendPasswordResetEmail(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success for missing email, got %#v", response)
	}
}

func TestResetPasswordRotatesPasswordAndClearsToken(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "reset-password-user",
		Password:    "password123",
		DisplayName: "Reset Password User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		Email:       "reset@example.com",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	token := "reset-token"
	identityapp.RegisterVerificationCodeWithKey(user.Email, token, identityapp.PasswordResetPurpose)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/reset", map[string]any{
		"email": user.Email,
		"token": token,
	}, 0)
	ResetPassword(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected password reset success, got %#v", response)
	}

	var temporaryPassword string
	if err := platformencoding.Unmarshal(response.Data, &temporaryPassword); err != nil {
		t.Fatalf("failed to decode temporary password: %v", err)
	}
	if temporaryPassword == "" {
		t.Fatal("expected non-empty temporary password")
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !platformsecurity.ValidatePasswordAndHash(temporaryPassword, reloaded.Password) {
		t.Fatalf("expected persisted temporary password, got %q", reloaded.Password)
	}
	if identityapp.VerifyCodeWithKey(user.Email, token, identityapp.PasswordResetPurpose) {
		t.Fatal("expected password reset token to be cleared")
	}
}

func TestResetPasswordRejectsExpiredToken(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/reset", map[string]any{
		"email": "reset@example.com",
		"token": "missing",
	}, 0)
	ResetPassword(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected password reset failure, got %#v", response)
	}
	if response.Message != "重置链接非法或已过期" {
		t.Fatalf("expected invalid reset token message, got %q", response.Message)
	}
}

func stubIdentitySendEmail(stub func(subject string, receiver string, content string) error) func() {
	original := identityapp.SetPasswordRecoveryEmailSenderForTest(stub)
	return func() {
		identityapp.SetPasswordRecoveryEmailSenderForTest(original)
	}
}

func TestSendPasswordResetEmailSwallowsMailerErrors(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "reset-email-user",
		Password:    "password123",
		DisplayName: "Reset Email User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		Email:       "reset-mail@example.com",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	restore := stubIdentitySendEmail(func(subject string, receiver string, content string) error {
		return errors.New("smtp down")
	})
	defer restore()

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/reset_password?email=reset-mail@example.com", nil, 0)
	SendPasswordResetEmail(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success despite mail failure, got %#v", response)
	}
}
