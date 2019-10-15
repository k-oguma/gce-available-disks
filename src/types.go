package src

import (
	"encoding/csv"
	"github.com/patrickmn/go-cache"
	"time"
)

type LogConfigure struct {
	maxLogEntries int
	diskName string
	projID string
    IterCache *cache.Cache
	timeout time.Duration
	isActivityLog bool
	isCSV	bool
	isSilent bool
	requireKeys []string
	parentKey string
	writer *csv.Writer
}

type ActivityPayload struct {
	AuthenticationInfo struct {
		PrincipalEmail string `json:"principal_email"`
	} `json:"authentication_info"`

	ResourceName string `json:"resource_name"`
}
