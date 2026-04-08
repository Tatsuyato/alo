package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var allowedKinds = map[string]bool{
	"deployment":  true,
	"statefulset": true,
	"daemonset":   true,
	"service":     true,
	"configmap":   true,
	"secret":      true,
	"ingress":     true,
	"job":         true,
	"cronjob":     true,
}

func (a *App) canAccessNamespace(p *Principal, namespace string) bool {
	if containsString(p.Namespaces, "*") {
		return true
	}
	return containsString(p.Namespaces, namespace)
}

func buildKubectlCommand(req commandRequest, project Project) ([]string, string, string, error) {
	ns := req.Namespace
	if ns == "" {
		ns = project.Namespace
	}
	if ns == "" {
		ns = defaultNamespace
	}

	var args []string
	stdinData := ""

	if project.ServiceAccount != "" {
		args = append(args, "--as=system:serviceaccount:"+ns+":"+project.ServiceAccount)
	}

	switch req.Action {
	case "create", "edit":
		args = append(args, "apply", "-f", "-", "-n", ns)
		stdinData = req.Manifest
	case "delete":
		k := strings.ToLower(strings.TrimSpace(req.Kind))
		if !allowedKinds[k] {
			return nil, "", "", fmt.Errorf("kind %q is not allowed", req.Kind)
		}
		args = append(args, "delete", k, req.Name, "-n", ns, "--ignore-not-found")
	default:
		return nil, "", "", fmt.Errorf("unsupported action %q", req.Action)
	}

	return args, "kubectl " + strings.Join(args, " "), stdinData, nil
}

func runKubectl(ctx context.Context, args []string, stdinData string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (a *App) machineStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	hostname, _ := os.Hostname()
	loadAvg := runQuickCommand("cat", "/proc/loadavg")
	uptime := runQuickCommand("uptime", "-p")
	dockerInfo := runQuickCommand("docker", "--version")

	writeJSON(w, 200, machineStatus{
		OK:         true,
		Hostname:   hostname,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUCount:   runtime.NumCPU(),
		GoVersion:  runtime.Version(),
		LoadAvg:    strings.TrimSpace(loadAvg),
		Uptime:     strings.TrimSpace(uptime),
		DockerInfo: strings.TrimSpace(dockerInfo),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

func runQuickCommand(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

func (a *App) logsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, 401, "missing principal")
		return
	}
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if ns == "" {
		ns = defaultNamespace
	}
	if !a.canAccessNamespace(principal, ns) {
		writeError(w, 403, "namespace access denied")
		return
	}
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if service == "" {
		writeError(w, 400, "service is required")
		return
	}
	tail := strings.TrimSpace(r.URL.Query().Get("tail"))
	if tail == "" {
		tail = "100"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out, err := runKubectl(ctx, []string{"logs", "-n", ns, "-l", "app=" + service, "--tail", tail}, "")
	if err != nil {
		writeError(w, 502, out+"\n"+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "namespace": ns, "service": service, "logs": out})
}

func (a *App) k8sStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "method not allowed")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, 401, "missing principal")
		return
	}
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if ns == "" {
		ns = defaultNamespace
	}
	if !a.canAccessNamespace(principal, ns) {
		writeError(w, 403, "namespace access denied")
		return
	}
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if service == "" {
		writeError(w, 400, "service is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	podJSON, err := runKubectl(ctx, []string{"get", "pods", "-n", ns, "-l", "app=" + service, "-o", "json"}, "")
	if err != nil {
		writeError(w, 502, podJSON+"\n"+err.Error())
		return
	}

	var parsed map[string]any
	if e := json.Unmarshal([]byte(podJSON), &parsed); e != nil {
		writeError(w, 502, "failed to parse pod status json")
		return
	}
	top, _ := runKubectl(ctx, []string{"top", "pod", "-n", ns, "-l", "app=" + service, "--no-headers"}, "")
	writeJSON(w, 200, map[string]any{
		"ok":        true,
		"namespace": ns,
		"service":   service,
		"pods":      parsed,
		"metrics":   strings.TrimSpace(top),
	})
}
