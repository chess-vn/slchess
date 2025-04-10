package dtos

type ServerStatusResponse struct {
	ActiveMatches int32 `json:"activeMatches"`
	CanAccept     bool  `json:"canAccept"`
	MaxMatches    int32 `json:"maxMatches"`
}
