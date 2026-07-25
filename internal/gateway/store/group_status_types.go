package store

type GroupModelRequestBucket struct {
	GroupName    string `gorm:"column:group_name"`
	ModelName    string `gorm:"column:model_name"`
	BucketIndex  int64  `gorm:"column:bucket_index"`
	RequestCount int64  `gorm:"column:request_count"`
	SuccessCount int64  `gorm:"column:success_count"`
}
