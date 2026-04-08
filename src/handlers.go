package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check environment variables
	envStatus := map[string]any{
		"PORT":             os.Getenv("PORT"),
		"DB_PATH":          os.Getenv("DB_PATH"),
		"API_KEY_ADMIN":    boolToString(os.Getenv("API_KEY_ADMIN") != ""),
		"API_KEY_DEPLOYER": boolToString(os.Getenv("API_KEY_DEPLOYER") != ""),
		"JWT_SECRET":       boolToString(os.Getenv("JWT_SECRET") != ""),
	}

	// Check kubectl availability
	kubectlAvailable, kubectlVersion := checkKubectl()

	// Check kubernetes cluster
	var clusterInfo map[string]any
	if kubectlAvailable {
		clusterInfo = getK8sClusterInfo()
	} else {
		clusterInfo = map[string]any{
			"available": false,
			"message":   "kubectl not found or not configured",
		}
	}

	// Check database
	dbHealthy := true
	if err := a.db.Ping(); err != nil {
		dbHealthy = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"service":     "alo-backend",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"environment": envStatus,
		"kubernetes": map[string]any{
			"kubectl_available": kubectlAvailable,
			"kubectl_version":   kubectlVersion,
			"cluster":           clusterInfo,
		},
		"database": map[string]any{
			"healthy": dbHealthy,
		},
	})
}

func (a *App) projectsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req createProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid json")
			return
		}
		if req.Name == "" || req.Repo == "" || req.Environment == "" {
			writeError(w, 400, "name, repo, environment are required")
			return
		}
		if req.Namespace == "" {
			req.Namespace = defaultNamespace
		}
		project, err := createProject(a.db, req)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true, "project": project})
	case http.MethodGet:
		projects, err := listProjects(a.db)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "projects": projects})
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (a *App) servicesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req createServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid json")
			return
		}
		if req.ProjectID == 0 || req.Name == "" || req.Image == "" {
			writeError(w, 400, "projectId, name and image are required")
			return
		}
		if _, err := getProject(a.db, req.ProjectID); err != nil {
			writeError(w, 404, "project not found")
			return
		}
		svc, err := createService(a.db, req)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true, "service": svc})
	case http.MethodGet:
		projectIDStr := strings.TrimSpace(r.URL.Query().Get("projectId"))
		projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
		if projectID == 0 {
			writeError(w, 400, "projectId is required")
			return
		}
		services, err := listServices(a.db, projectID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "services": services})
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (a *App) commandsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}

	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, 401, "missing principal")
		return
	}

	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid json body")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action == "" {
		writeError(w, 400, "action is required")
		return
	}
	if req.ProjectID == 0 {
		writeError(w, 400, "projectId is required")
		return
	}

	project, err := getProject(a.db, req.ProjectID)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}

	ns := req.Namespace
	if ns == "" {
		ns = project.Namespace
	}
	if !a.canAccessNamespace(principal, ns) {
		writeError(w, 403, "namespace access denied")
		return
	}

	if (req.Action == "create" || req.Action == "edit") && strings.TrimSpace(req.Manifest) == "" {
		writeError(w, 400, "manifest is required for create/edit")
		return
	}
	if req.Action == "delete" && (strings.TrimSpace(req.Kind) == "" || strings.TrimSpace(req.Name) == "") {
		writeError(w, 400, "kind and name are required for delete")
		return
	}

	if req.Action == "create" || req.Action == "edit" {
		if err := validateManifest(req.Manifest, ResourcePolicy{MaxCPU: project.CPULimit, MaxMemory: project.MemoryLimit}); err != nil {
			writeError(w, 400, "manifest validation failed: "+err.Error())
			return
		}
	}

	if req.DryRun {
		args, label, _, err := buildKubectlCommand(req, project)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "command": label, "args": args, "dryRun": true})
		return
	}

	job := a.queue.Enqueue("k8s_command", map[string]any{"request": req, "project": project}, 2)
	a.queue.RegisterRunner("k8s_command", a.runK8sCommandJob)
	writeJSON(w, 202, commandResponse{OK: true, JobID: job.ID, Message: "job queued"})
}

func (a *App) buildsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	var req buildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid json")
		return
	}
	if req.ProjectID == 0 || req.ServiceID == 0 {
		writeError(w, 400, "projectId and serviceId are required")
		return
	}
	project, err := getProject(a.db, req.ProjectID)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}
	service, err := getService(a.db, req.ServiceID)
	if err != nil {
		writeError(w, 404, "service not found")
		return
	}
	if req.Tag == "" {
		req.Tag = fmt.Sprintf("%d", time.Now().Unix())
	}
	payload := map[string]any{"project": project, "service": service, "request": req}
	a.queue.RegisterRunner("build_deploy", a.runBuildDeployJob)
	job := a.queue.Enqueue("build_deploy", payload, 1)
	writeJSON(w, 202, map[string]any{"ok": true, "jobId": job.ID, "message": "build/deploy queued"})
}

func (a *App) jobByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if id == "" {
		writeError(w, 400, "job id is required")
		return
	}
	job, ok := a.queue.Get(id)
	if !ok {
		writeError(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "job": job})
}

func (a *App) runK8sCommandJob(job *Job) {
	payload, ok := job.Payload.(map[string]any)
	if !ok {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "invalid job payload"
		})
		return
	}

	reqMap, ok := payload["request"].(map[string]any)
	if !ok {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "invalid command payload"
		})
		return
	}

	projectMap, ok := payload["project"].(map[string]any)
	if !ok {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "invalid project payload"
		})
		return
	}

	var req commandRequest
	bReq, _ := json.Marshal(reqMap)
	_ = json.Unmarshal(bReq, &req)
	var project Project
	bProj, _ := json.Marshal(projectMap)
	_ = json.Unmarshal(bProj, &project)

	args, label, stdinData, err := buildKubectlCommand(req, project)
	if err != nil {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = err.Error()
		})
		return
	}

	var lastErr error
	var out string
	for i := 0; i <= job.MaxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, lastErr = runKubectl(ctx, args, stdinData)
		cancel()
		if lastErr == nil {
			a.queue.update(job.ID, func(j *Job) {
				j.Status = JobDone
				j.Output = out
			})
			return
		}
		a.queue.update(job.ID, func(j *Job) {
			if i < job.MaxRetries {
				j.Status = JobRetrying
			}
			j.Output = out
			j.Error = lastErr.Error()
		})
	}

	if req.Action == "edit" || req.Action == "create" {
		if req.Name != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _ = runKubectl(ctx, []string{"rollout", "undo", req.Kind + "/" + req.Name, "-n", project.Namespace}, "")
			cancel()
			a.queue.update(job.ID, func(j *Job) {
				j.Status = JobRolledBack
				j.Error = "rolled back due to failure: " + lastErr.Error()
			})
			return
		}
	}

	a.queue.update(job.ID, func(j *Job) {
		j.Status = JobFailed
		j.Error = lastErr.Error()
		j.Output = out + "\n" + label
	})
}

func (a *App) runBuildDeployJob(job *Job) {
	payload, ok := job.Payload.(map[string]any)
	if !ok {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "invalid job payload"
		})
		return
	}

	var project Project
	bProj, _ := json.Marshal(payload["project"])
	_ = json.Unmarshal(bProj, &project)
	var service Service
	bSvc, _ := json.Marshal(payload["service"])
	_ = json.Unmarshal(bSvc, &service)
	var req buildRequest
	bReq, _ := json.Marshal(payload["request"])
	_ = json.Unmarshal(bReq, &req)

	workDir, err := os.MkdirTemp("", "alo-build-*")
	if err != nil {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = err.Error()
		})
		return
	}
	defer os.RemoveAll(workDir)

	repoDir := filepath.Join(workDir, "repo")
	cloneCmd := exec.Command("git", "clone", "--depth", "1", project.Repo, repoDir)
	if req.GitRef != "" {
		cloneCmd = exec.Command("git", "clone", "--depth", "1", "--branch", req.GitRef, project.Repo, repoDir)
	}
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "git clone failed: " + err.Error()
			j.Output = string(out)
		})
		return
	}

	imageTag := service.Image + ":" + req.Tag
	buildCmd := exec.Command("docker", "build", "-t", imageTag, ".")
	buildCmd.Dir = repoDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "docker build failed: " + err.Error()
			j.Output = string(out)
		})
		return
	}

	pushCmd := exec.Command("docker", "push", imageTag)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobFailed
			j.Error = "docker push failed: " + err.Error()
			j.Output = string(out)
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	out, err := runKubectl(ctx, []string{"set", "image", "deployment/" + service.Name, service.Name + "=" + imageTag, "-n", project.Namespace}, "")
	if err != nil {
		_, _ = runKubectl(ctx, []string{"rollout", "undo", "deployment/" + service.Name, "-n", project.Namespace}, "")
		a.queue.update(job.ID, func(j *Job) {
			j.Status = JobRolledBack
			j.Error = "deploy failed and rollback applied: " + err.Error()
			j.Output = out
		})
		return
	}

	a.queue.update(job.ID, func(j *Job) {
		j.Status = JobDone
		j.Output = "image deployed: " + imageTag + "\n" + out
	})
}
