package schema

import (
	"crypto/rand"
	"math/big"

	"gorm.io/gorm"
)

const UserNameMaxLength = 20

const (
	ExternalUserIDLength = 6
	externalUserIDChars  = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

// GenerateExternalUserID returns a human-readable, fixed-length public user ID.
func GenerateExternalUserID() (string, error) {
	chars := make([]byte, ExternalUserIDLength)
	limit := big.NewInt(int64(len(externalUserIDChars)))
	for index := range chars {
		value, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		chars[index] = externalUserIDChars[value.Int64()]
	}
	return string(chars), nil
}

// User is the persisted identity and account record.
type User struct {
	Id                           int            `json:"id"`
	ExternalId                   string         `json:"external_id" gorm:"type:char(6);default:null;uniqueIndex:idx_users_external_id"`
	Username                     string         `json:"username" gorm:"unique;index" validate:"max=20"`
	Password                     string         `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	OriginalPassword             string         `json:"original_password" gorm:"-:all"`
	DisplayName                  string         `json:"display_name" gorm:"index" validate:"max=20"`
	Role                         int            `json:"role" gorm:"type:int;default:1"`
	Status                       int            `json:"status" gorm:"type:int;default:1"`
	Email                        string         `json:"email" gorm:"index" validate:"max=50"`
	GitHubId                     string         `json:"github_id" gorm:"column:github_id;index"`
	DiscordId                    string         `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId                       string         `json:"oidc_id" gorm:"column:oidc_id;index"`
	WeChatId                     string         `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId                   string         `json:"telegram_id" gorm:"column:telegram_id;index"`
	VerificationCode             string         `json:"verification_code" gorm:"-:all"`
	AccessToken                  *string        `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"`
	Quota                        int            `json:"quota" gorm:"type:int;default:0"`
	UsedQuota                    int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"`
	RequestCount                 int            `json:"request_count" gorm:"type:int;default:0;"`
	Group                        string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	DeletedAt                    gorm.DeletedAt `gorm:"index"`
	LinuxDOId                    string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting                      string         `json:"setting" gorm:"type:text;column:setting"`
	Remark                       string         `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer               string         `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	CreatedAt                    int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	LastLoginAt                  int64          `json:"last_login_at" gorm:"default:0;column:last_login_at"`
	CurrentSubscriptionStatus    string         `json:"current_subscription_status,omitempty" gorm:"-"`
	CurrentSubscriptionPlanTitle string         `json:"current_subscription_plan_title,omitempty" gorm:"-"`
	CurrentSubscriptionEndTime   int64          `json:"current_subscription_end_time,omitempty" gorm:"-"`
}

type UserBase struct {
	Id         int    `json:"id"`
	ExternalId string `json:"external_id"`
	Group      string `json:"group"`
	Email      string `json:"email"`
	Quota      int    `json:"quota"`
	Status     int    `json:"status"`
	Username   string `json:"username"`
	Setting    string `json:"setting"`
}

func (user *User) ToBaseUser() *UserBase {
	return &UserBase{
		Id:         user.Id,
		ExternalId: user.ExternalId,
		Group:      user.Group,
		Quota:      user.Quota,
		Status:     user.Status,
		Username:   user.Username,
		Setting:    user.Setting,
		Email:      user.Email,
	}
}

func (user *User) GetAccessToken() string {
	if user.AccessToken == nil {
		return ""
	}
	return *user.AccessToken
}

func (user *User) SetAccessToken(token string) {
	user.AccessToken = &token
}
