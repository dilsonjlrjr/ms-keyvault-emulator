package domain

const RecoveryLevelPurgeable = "Recoverable+Purgeable"

type Attributes struct {
	Enabled         bool  `json:"enabled"`
	NotBefore       *int64 `json:"nbf,omitempty"`
	Expires         *int64 `json:"exp,omitempty"`
	Created         int64  `json:"created"`
	Updated         int64  `json:"updated"`
	RecoveryLevel   string `json:"recoveryLevel"`
	RecoverableDays int    `json:"recoverableDays,omitempty"`
}
