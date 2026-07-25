package schema

import "gorm.io/gorm"

// RoutePool is a root-managed, group-scoped automatic routing pool.
type RoutePool struct {
	ID      int64  `json:"id" gorm:"primaryKey"`
	Name    string `json:"name" gorm:"size:128;not null"`
	Group   string `json:"group" gorm:"size:64;not null;uniqueIndex:uq_route_pool_group_deleted"`
	Enabled bool   `json:"enabled" gorm:"not null;default:true;index"`
	// AutoDiscover keeps the pool aligned with channels already assigned to its
	// group. Existing pools remain opt-in until an administrator saves them in
	// the consolidated channel view.
	AutoDiscover bool           `json:"auto_discover" gorm:"not null;default:false"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"uniqueIndex:uq_route_pool_group_deleted"`
}

// RoutePoolMember supplies a channel's private procurement cost to a pool.
type RoutePoolMember struct {
	ID                 int64          `json:"id" gorm:"primaryKey"`
	RoutePoolID        int64          `json:"route_pool_id" gorm:"not null;uniqueIndex:uq_route_pool_channel;index"`
	ChannelID          int            `json:"channel_id" gorm:"not null;uniqueIndex:uq_route_pool_channel;index"`
	CostMultiplier     float64        `json:"cost_multiplier" gorm:"not null;default:1"`
	ModelCostOverrides string         `json:"model_cost_overrides" gorm:"type:text;not null;default:'{}'"`
	Enabled            bool           `json:"enabled" gorm:"not null;default:true;index"`
	DeletedAt          gorm.DeletedAt `json:"-"`
}
