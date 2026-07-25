package domain

type LogListQuery struct {
	LogType           int
	StartTimestamp    int64
	EndTimestamp      int64
	Username          string
	TokenName         string
	ModelName         string
	Channel           int
	Group             string
	RequestID         string
	UpstreamRequestID string
	StartIdx          int
	PageSize          int
}
