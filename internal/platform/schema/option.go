package schema

// Option stores a persisted runtime configuration value.
type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}
