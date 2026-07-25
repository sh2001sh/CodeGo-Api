package schema

import "gorm.io/gorm"

const (
	RedemptionTypeQuota        = "quota"
	RedemptionTypeSubscription = "subscription"
)

// Redemption is a redeemable code managed by administrators.
type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	RedeemType   string         `json:"redeem_type" gorm:"type:varchar(32);not null;default:'quota';index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	PlanId       int            `json:"plan_id" gorm:"default:0;index"`
	PlanTitle    string         `json:"plan_title" gorm:"type:varchar(128);default:''"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"`
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"`
}
