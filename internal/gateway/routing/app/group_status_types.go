package app

type GroupModelStatusSummary struct {
	Group           string `json:"group"`
	Model           string `json:"model"`
	Status          string `json:"status"`
	Channels        int    `json:"-"`
	EnabledChannels int    `json:"-"`
}
