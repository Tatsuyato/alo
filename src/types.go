package main

import "time"

const defaultNamespace = "default"

type Permission string

const (
	PermExecuteCommand Permission = "execute_command"
	PermReadStatus     Permission = "read_status"
	PermReadLogs       Permission = "read_logs"
	PermManageProject  Permission = "manage_project"
	PermBuildDeploy    Permission = "build_deploy"
)

type Principal struct {
	Subject     string       `json:"sub"`
	Role        string       `json:"role"`
	Namespaces  []string     `json:"namespaces"`
	Permissions []Permission `json:"permissions"`
}

type commandRequest struct {
	ProjectID int64  `json:"projectId"`
	ServiceID int64  `json:"serviceId"`
	Action    string `json:"action"`
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Manifest  string `json:"manifest,omitempty"`
	DryRun    bool   `json:"dryRun,omitempty"`
}

type commandResponse struct {
	OK         bool   `json:"ok"`
	JobID      string `json:"jobId"`
	Message    string `json:"message"`
	Command    string `json:"command,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

type errorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type machineStatus struct {
	OK         bool   `json:"ok"`
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	CPUCount   int    `json:"cpuCount"`
	GoVersion  string `json:"goVersion"`
	LoadAvg    string `json:"loadAvg,omitempty"`
	Uptime     string `json:"uptime,omitempty"`
	DockerInfo string `json:"dockerInfo,omitempty"`
	Timestamp  string `json:"timestamp"`
}

type createProjectRequest struct {
	Name           string `json:"name"`
	Repo           string `json:"repo"`
	Environment    string `json:"environment"`
	Namespace      string `json:"namespace"`
	CPULimit       string `json:"cpuLimit"`
	MemoryLimit    string `json:"memoryLimit"`
	ServiceAccount string `json:"serviceAccount"`
}

type createServiceRequest struct {
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Replicas  int    `json:"replicas"`
}

type buildRequest struct {
	ProjectID int64  `json:"projectId"`
	ServiceID int64  `json:"serviceId"`
	GitRef    string `json:"gitRef"`
	Tag       string `json:"tag"`
}

type Project struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Repo           string    `json:"repo"`
	Environment    string    `json:"environment"`
	Namespace      string    `json:"namespace"`
	CPULimit       string    `json:"cpuLimit"`
	MemoryLimit    string    `json:"memoryLimit"`
	ServiceAccount string    `json:"serviceAccount"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Service struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"projectId"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Replicas  int       `json:"replicas"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobRunning    JobStatus = "running"
	JobDone       JobStatus = "done"
	JobFailed     JobStatus = "failed"
	JobRetrying   JobStatus = "retrying"
	JobRolledBack JobStatus = "rolled_back"
)

type Job struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Status     JobStatus `json:"status"`
	Attempts   int       `json:"attempts"`
	MaxRetries int       `json:"maxRetries"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Payload    any       `json:"payload,omitempty"`
}
