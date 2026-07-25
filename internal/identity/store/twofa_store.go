package store

import (
	"errors"
	"fmt"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"time"

	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"gorm.io/gorm"
)

func LoadTwoFAByUserID(userID int) (*identitydomain.TwoFA, error) {
	if userID == 0 {
		return nil, errors.New("用户ID不能为空")
	}

	var twoFA identitydomain.TwoFA
	err := platformdb.DB.Where("user_id = ?", userID).First(&twoFA).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &twoFA, nil
}

func IsTwoFAEnabled(userID int) bool {
	twoFA, err := LoadTwoFAByUserID(userID)
	if err != nil || twoFA == nil {
		return false
	}
	return twoFA.IsEnabled
}

func CreateTwoFA(twoFA *identitydomain.TwoFA) error {
	if twoFA == nil {
		return errors.New("2FA记录不能为空")
	}

	var existing identitydomain.TwoFA
	err := platformdb.DB.Where("user_id = ?", twoFA.UserId).First(&existing).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else {
		return errors.New("用户已存在2FA设置")
	}

	var user identityschema.User
	if err := platformdb.DB.First(&user, twoFA.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	return platformdb.DB.Create(twoFA).Error
}

func DeleteTwoFA(twoFA *identitydomain.TwoFA) error {
	if twoFA == nil || twoFA.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}

	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", twoFA.UserId).Delete(&identitydomain.TwoFABackupCode{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(twoFA).Error
	})
}

func DeleteTwoFAByUserID(userID int) error {
	twoFA, err := LoadTwoFAByUserID(userID)
	if err != nil {
		return err
	}
	if twoFA == nil {
		return identitydomain.ErrTwoFANotEnabled
	}
	return DeleteTwoFA(twoFA)
}

func EnableTwoFA(twoFA *identitydomain.TwoFA) error {
	if twoFA == nil {
		return errors.New("2FA记录不能为空")
	}
	twoFA.IsEnabled = true
	twoFA.FailedAttempts = 0
	twoFA.LockedUntil = nil
	return updateTwoFA(twoFA)
}

func ReplaceTwoFABackupCodes(userID int, codes []string) error {
	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&identitydomain.TwoFABackupCode{}).Error; err != nil {
			return err
		}

		for _, code := range codes {
			hashedCode, err := platformsecurity.HashBackupCode(code)
			if err != nil {
				return err
			}

			backupCode := identitydomain.TwoFABackupCode{
				UserId:   userID,
				CodeHash: hashedCode,
				IsUsed:   false,
			}
			if err := tx.Create(&backupCode).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func CountUnusedTwoFABackupCodes(userID int) (int, error) {
	var count int64
	err := platformdb.DB.Model(&identitydomain.TwoFABackupCode{}).Where("user_id = ? AND is_used = false", userID).Count(&count).Error
	return int(count), err
}

func LoadTwoFAStats() (map[string]any, error) {
	var totalUsers, enabledUsers int64
	if err := platformdb.DB.Model(&identityschema.User{}).Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	if err := platformdb.DB.Model(&identitydomain.TwoFA{}).Where("is_enabled = true").Count(&enabledUsers).Error; err != nil {
		return nil, err
	}

	enabledRate := float64(0)
	if totalUsers > 0 {
		enabledRate = float64(enabledUsers) / float64(totalUsers) * 100
	}

	return map[string]any{
		"total_users":   totalUsers,
		"enabled_users": enabledUsers,
		"enabled_rate":  fmt.Sprintf("%.1f%%", enabledRate),
	}, nil
}

func ValidateTwoFATOTPAndTrackUsage(twoFA *identitydomain.TwoFA, code string) (bool, error) {
	if twoFA == nil {
		return false, errors.New("2FA记录不能为空")
	}
	if twoFA.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", twoFA.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	if !platformsecurity.ValidateTOTPCode(twoFA.Secret, code) {
		if err := incrementTwoFAFailedAttempts(twoFA); err != nil {
			platformobservability.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	now := time.Now()
	twoFA.FailedAttempts = 0
	twoFA.LockedUntil = nil
	twoFA.LastUsedAt = &now
	if err := updateTwoFA(twoFA); err != nil {
		platformobservability.SysLog("更新2FA使用记录失败: " + err.Error())
	}
	return true, nil
}

func ValidateTwoFABackupCodeAndTrackUsage(twoFA *identitydomain.TwoFA, code string) (bool, error) {
	if twoFA == nil {
		return false, errors.New("2FA记录不能为空")
	}
	if twoFA.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", twoFA.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	valid, err := validateAndConsumeTwoFABackupCode(twoFA.UserId, code)
	if err != nil {
		return false, err
	}
	if !valid {
		if err := incrementTwoFAFailedAttempts(twoFA); err != nil {
			platformobservability.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	now := time.Now()
	twoFA.FailedAttempts = 0
	twoFA.LockedUntil = nil
	twoFA.LastUsedAt = &now
	if err := updateTwoFA(twoFA); err != nil {
		platformobservability.SysLog("更新2FA使用记录失败: " + err.Error())
	}
	return true, nil
}

func updateTwoFA(twoFA *identitydomain.TwoFA) error {
	if twoFA == nil || twoFA.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	return platformdb.DB.Save(twoFA).Error
}

func incrementTwoFAFailedAttempts(twoFA *identitydomain.TwoFA) error {
	twoFA.FailedAttempts++
	if twoFA.FailedAttempts >= platformsecurity.MaxFailAttempts {
		lockUntil := time.Now().Add(time.Duration(platformsecurity.LockoutDuration) * time.Second)
		twoFA.LockedUntil = &lockUntil
	}
	return updateTwoFA(twoFA)
}

func validateAndConsumeTwoFABackupCode(userID int, code string) (bool, error) {
	if !platformsecurity.ValidateBackupCode(code) {
		return false, errors.New("验证码或备用码不正确")
	}

	normalizedCode := platformsecurity.NormalizeBackupCode(code)

	var backupCodes []identitydomain.TwoFABackupCode
	if err := platformdb.DB.Where("user_id = ? AND is_used = false", userID).Find(&backupCodes).Error; err != nil {
		return false, err
	}

	for _, backupCode := range backupCodes {
		if platformsecurity.ValidatePasswordAndHash(normalizedCode, backupCode.CodeHash) {
			now := time.Now()
			backupCode.IsUsed = true
			backupCode.UsedAt = &now
			if err := platformdb.DB.Save(&backupCode).Error; err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}
