package app

import (
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
)

var (
	ErrTwoFAAlreadyEnabled             = errors.New("用户已启用2FA，请先禁用后重新设置")
	ErrTwoFASetupMissing               = errors.New("请先完成2FA初始化设置")
	ErrTwoFAAlreadyActive              = errors.New("2FA已经启用")
	ErrTwoFANotEnabled                 = errors.New("用户未启用2FA")
	ErrTwoFAInvalidCode                = errors.New("验证码或备用码错误，请重试")
	ErrTwoFASecretGenerationFailed     = errors.New("生成2FA密钥失败")
	ErrTwoFABackupCodeGenerationFailed = errors.New("生成备用码失败")
	ErrTwoFABackupCodeSaveFailed       = errors.New("保存备用码失败")
	ErrSessionExpired                  = errors.New("会话已过期，请重新登录")
	ErrSessionInvalid                  = errors.New("会话数据无效，请重新登录")
	ErrTwoFAUserNotFound               = errors.New("用户不存在")
	ErrTwoFAUserIDFormat               = errors.New("用户ID格式错误")
	ErrTwoFANoPermission               = errors.New("无权操作同级或更高级用户的2FA设置")
)

type SetupTwoFAResponse struct {
	Secret      string   `json:"secret"`
	QRCodeData  string   `json:"qr_code_data"`
	BackupCodes []string `json:"backup_codes"`
}

type TwoFAStatusResponse struct {
	Enabled              bool `json:"enabled"`
	Locked               bool `json:"locked"`
	BackupCodesRemaining int  `json:"backup_codes_remaining,omitempty"`
}

// LoadEnabledTwoFA returns the user's enabled 2FA record when present.
func LoadEnabledTwoFA(userID int) (*identitydomain.TwoFA, error) {
	twoFA, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return nil, err
	}
	if twoFA == nil || !twoFA.IsEnabled {
		return nil, nil
	}
	return twoFA, nil
}

// InitializeTwoFA creates a disabled 2FA record and backup codes for the user.
func InitializeTwoFA(userID int) (*SetupTwoFAResponse, error) {
	existing, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.IsEnabled {
		return nil, ErrTwoFAAlreadyEnabled
	}
	if existing != nil && !existing.IsEnabled {
		if err := identitystore.DeleteTwoFA(existing); err != nil {
			return nil, err
		}
	}

	user, err := LoadUserByID(userID, false)
	if err != nil {
		return nil, err
	}

	key, err := platformsecurity.GenerateTOTPSecret(user.Username, platformconfig.SystemName)
	if err != nil {
		return nil, ErrTwoFASecretGenerationFailed
	}

	backupCodes, err := platformsecurity.GenerateBackupCodes()
	if err != nil {
		return nil, ErrTwoFABackupCodeGenerationFailed
	}

	twoFA := &identitydomain.TwoFA{
		UserId:    userID,
		Secret:    key.Secret(),
		IsEnabled: false,
	}
	if err := identitystore.CreateTwoFA(twoFA); err != nil {
		return nil, err
	}
	if err := identitystore.ReplaceTwoFABackupCodes(userID, backupCodes); err != nil {
		return nil, ErrTwoFABackupCodeSaveFailed
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, "开始设置两步验证")
	return &SetupTwoFAResponse{
		Secret:      key.Secret(),
		QRCodeData:  platformsecurity.GenerateQRCodeData(key.Secret(), user.Username, platformconfig.SystemName),
		BackupCodes: backupCodes,
	}, nil
}

// EnableTwoFA validates the setup code and enables 2FA.
func EnableTwoFA(userID int, code string) error {
	twoFA, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return err
	}
	if twoFA == nil {
		return ErrTwoFASetupMissing
	}
	if twoFA.IsEnabled {
		return ErrTwoFAAlreadyActive
	}

	cleanCode, err := platformsecurity.ValidateNumericCode(code)
	if err != nil {
		return err
	}
	if !platformsecurity.ValidateTOTPCode(twoFA.Secret, cleanCode) {
		return ErrTwoFAInvalidCode
	}
	if err := identitystore.EnableTwoFA(twoFA); err != nil {
		return err
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, "成功启用两步验证")
	return nil
}

// DisableTwoFA validates a TOTP or backup code and removes the user's 2FA setup.
func DisableTwoFA(userID int, code string) error {
	twoFA, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return err
	}
	if twoFA == nil || !twoFA.IsEnabled {
		return ErrTwoFANotEnabled
	}

	if err := validateTwoFACodeOrBackup(twoFA, code); err != nil {
		return err
	}
	if err := identitystore.DeleteTwoFAByUserID(userID); err != nil {
		return err
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, "禁用两步验证")
	return nil
}

// LoadTwoFAStatus returns the user's current 2FA state.
func LoadTwoFAStatus(userID int) (*TwoFAStatusResponse, error) {
	twoFA, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return nil, err
	}

	status := &TwoFAStatusResponse{}
	if twoFA == nil {
		return status, nil
	}

	status.Enabled = twoFA.IsEnabled
	status.Locked = twoFA.IsLocked()
	if twoFA.IsEnabled {
		backupCount, err := identitystore.CountUnusedTwoFABackupCodes(userID)
		if err != nil {
			platformobservability.SysLog("获取备用码数量失败: " + err.Error())
		} else {
			status.BackupCodesRemaining = backupCount
		}
	}
	return status, nil
}

// RegenerateTwoFABackupCodes validates the user and replaces their backup codes.
func RegenerateTwoFABackupCodes(userID int, code string) ([]string, error) {
	twoFA, err := identitystore.LoadTwoFAByUserID(userID)
	if err != nil {
		return nil, err
	}
	if twoFA == nil || !twoFA.IsEnabled {
		return nil, ErrTwoFANotEnabled
	}

	cleanCode, err := platformsecurity.ValidateNumericCode(code)
	if err != nil {
		return nil, err
	}
	valid, err := identitystore.ValidateTwoFATOTPAndTrackUsage(twoFA, cleanCode)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrTwoFAInvalidCode
	}

	backupCodes, err := platformsecurity.GenerateBackupCodes()
	if err != nil {
		return nil, ErrTwoFABackupCodeGenerationFailed
	}
	if err := identitystore.ReplaceTwoFABackupCodes(userID, backupCodes); err != nil {
		return nil, ErrTwoFABackupCodeSaveFailed
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, "重新生成两步验证备用码")
	return backupCodes, nil
}

// VerifyTwoFALogin validates the pending user's 2FA code and returns the user to log in.
func VerifyTwoFALogin(userID int, code string) (*identityschema.User, error) {
	user, err := LoadUserByID(userID, false)
	if err != nil {
		return nil, ErrTwoFAUserNotFound
	}

	twoFA, err := identitystore.LoadTwoFAByUserID(user.Id)
	if err != nil {
		return nil, err
	}
	if twoFA == nil || !twoFA.IsEnabled {
		return nil, ErrTwoFANotEnabled
	}

	if err := validateTwoFACodeOrBackup(twoFA, code); err != nil {
		return nil, err
	}
	return user, nil
}

// LoadTwoFAStats returns the admin-facing aggregate 2FA adoption metrics.
func LoadTwoFAStats() (map[string]any, error) {
	return identitystore.LoadTwoFAStats()
}

// ForceDisableTwoFA allows an authorized admin to remove 2FA from a target user.
func ForceDisableTwoFA(targetUserID int, actorID int, actorRole int, actorUsername string) error {
	targetUser, err := LoadUserByID(targetUserID, false)
	if err != nil {
		return err
	}
	if actorRole <= targetUser.Role && actorRole != constant.RoleRootUser {
		return ErrTwoFANoPermission
	}

	if err := identitystore.DeleteTwoFAByUserID(targetUserID); err != nil {
		return err
	}

	adminInfo := map[string]any{
		"admin_id":       actorID,
		"admin_username": actorUsername,
	}
	auditapp.RecordLogWithAdminInfo(targetUserID, auditschema.LogTypeManage, "管理员强制禁用了用户的两步验证", adminInfo)
	return nil
}

func validateTwoFACodeOrBackup(twoFA *identitydomain.TwoFA, code string) error {
	cleanCode, numericErr := platformsecurity.ValidateNumericCode(code)
	isValidTOTP := false
	isValidBackup := false

	if numericErr == nil {
		isValidTOTP, _ = identitystore.ValidateTwoFATOTPAndTrackUsage(twoFA, cleanCode)
	}
	if !isValidTOTP {
		var backupErr error
		isValidBackup, backupErr = identitystore.ValidateTwoFABackupCodeAndTrackUsage(twoFA, code)
		if backupErr != nil {
			return backupErr
		}
	}
	if !isValidTOTP && !isValidBackup {
		return ErrTwoFAInvalidCode
	}
	return nil
}

func ValidateTwoFACodeForSecurityVerification(twoFA *identitydomain.TwoFA, code string) bool {
	return validateTwoFACodeOrBackup(twoFA, code) == nil
}
