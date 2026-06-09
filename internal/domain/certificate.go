package domain

type CertVersion struct {
	Name             string
	Version          string
	CerDER           []byte
	X5T              string
	X5TS256          string
	BackingSecretVer string
	BackingKeyVer    string
	Policy           map[string]any
	Attributes       Attributes
}

type DeletedCert struct {
	CertVersion
	DeletedDate    int64  `json:"deletedDate"`
	ScheduledPurge int64  `json:"scheduledPurgeDate"`
	RecoveryID     string `json:"recoveryId"`
}
