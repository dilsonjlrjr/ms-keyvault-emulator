package domain

type KeyVersion struct {
	Name       string
	Version    string
	Kty        string
	Crv        string
	KeySize    int
	KeyOps     []string
	PubJWK     map[string]any // kty/n/e/crv/x/y/kid
	Attributes Attributes
}

type DeletedKey struct {
	KeyVersion
	DeletedDate    int64  `json:"deletedDate"`
	ScheduledPurge int64  `json:"scheduledPurgeDate"`
	RecoveryID     string `json:"recoveryId"`
}
