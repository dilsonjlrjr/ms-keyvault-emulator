package domain

type SecretVersion struct {
	Name        string
	Version     string
	Value       string
	ContentType string
	Tags        map[string]string
	Managed     bool
	Attributes  Attributes
}

type DeletedSecret struct {
	SecretVersion
	DeletedDate    int64  `json:"deletedDate"`
	ScheduledPurge int64  `json:"scheduledPurgeDate"`
	RecoveryID     string `json:"recoveryId"`
}
