package app

type UpstreamSource struct {
	Locale     string
	ModelsURL  string
	VendorsURL string
}

type UpstreamFetchError struct {
	Source UpstreamSource
	Err    error
}

func (e *UpstreamFetchError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

type UpstreamConflictField struct {
	Field    string `json:"field"`
	Local    any    `json:"local"`
	Upstream any    `json:"upstream"`
}

type UpstreamConflictItem struct {
	ModelName string                  `json:"model_name"`
	Fields    []UpstreamConflictField `json:"fields"`
}

type UpstreamPreviewResult struct {
	Missing   []string
	Conflicts []UpstreamConflictItem
	Source    UpstreamSource
}

type OverwriteField struct {
	ModelName string   `json:"model_name"`
	Fields    []string `json:"fields"`
}

type SyncRequest struct {
	Overwrite []OverwriteField `json:"overwrite"`
	Locale    string           `json:"locale"`
}

type SyncUpstreamResult struct {
	CreatedModels  int
	CreatedVendors int
	UpdatedModels  int
	SkippedModels  []string
	CreatedList    []string
	UpdatedList    []string
	Source         UpstreamSource
}
