package migrator

type Status int

const (
	OK Status = iota
	FAIL
)

const (
	RPCPort  = ":1234"
	FilePort = ":1235"
)

type MigrateRequest struct {
	UserName     string
	InstanceName string
	Target       string
}

type MigrateResponse struct {
	Status Status
}

type DisklessMigrateRequest struct {
	UserName     string
	InstanceName string
	Target       string
}

type DisklessMigrateResponse struct {
	Status Status
}

type LaunchPageServerRequest struct {
	UserName       string
	InstanceName   string
	CheckpointName string
	ImagePath      string
}

type LaunchPageServerResponse struct {
	Status Status
}

type RestartContainerRequest struct {
	UserName       string
	InstanceName   string
	CheckpointName string
	ImagePath      string
}

type RestartContainerResponse struct {
	Status Status
}

type RestoreRequest struct {
	UserName     string
	InstanceName string
}

type RestoreResponse struct {
	Status Status
}
