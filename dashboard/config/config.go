package config

const (
	UnixRange = 10
)

const (
	SERVICE_STATUS_DEFAULT        = 0
	SERVICE_STATUS_NO_WALLET_FILE = 1
	SERVICE_STATUS_NO_PASSWORD    = 2
	SERVICE_STATUS_RUNNING        = 3
)

var (
	IsInit   = false
	IsRemote = false
	Status   = SERVICE_STATUS_DEFAULT
)
