package store

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

func LoadUserByGitHubID(githubID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if githubID == "" {
		return nil, errors.New("GitHub id 为空！")
	}
	if err := platformdb.DB.Where(identityschema.User{GitHubId: githubID}).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func LoadUserByDiscordID(discordID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if discordID == "" {
		return nil, errors.New("discord id 为空！")
	}
	if err := platformdb.DB.Where(identityschema.User{DiscordId: discordID}).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func LoadUserByOIDCID(oidcID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if oidcID == "" {
		return nil, errors.New("oidc id 为空！")
	}
	if err := platformdb.DB.Where(identityschema.User{OidcId: oidcID}).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func LoadUserByWeChatID(wechatID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if wechatID == "" {
		return nil, errors.New("WeChat id 为空！")
	}
	if err := platformdb.DB.Where(identityschema.User{WeChatId: wechatID}).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func LoadUserByTelegramID(telegramID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if telegramID == "" {
		return nil, errors.New("Telegram id 为空！")
	}
	err := platformdb.DB.Where(identityschema.User{TelegramId: telegramID}).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("该 Telegram 账户未绑定")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func LoadUserByLinuxDOID(linuxDOID string) (*identityschema.User, error) {
	user := &identityschema.User{}
	if linuxDOID == "" {
		return nil, errors.New("linux do id is empty")
	}
	if err := platformdb.DB.Where("linux_do_id = ?", linuxDOID).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func IsWeChatIDTaken(wechatID string) bool {
	return platformdb.DB.Where("wechat_id = ?", wechatID).Find(&identityschema.User{}).RowsAffected == 1
}

func IsTelegramIDTaken(telegramID string) bool {
	return platformdb.DB.Where("telegram_id = ?", telegramID).Find(&identityschema.User{}).RowsAffected == 1
}
