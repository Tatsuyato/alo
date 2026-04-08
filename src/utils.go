package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{OK: false, Error: message})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func hasPermission(list []Permission, p Permission) bool {
	for _, item := range list {
		if item == p {
			return true
		}
	}
	return false
}

func boolToString(b bool) string {
	if b {
		return "set"
	}
	return "not set"
}

func checkKubectl() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--client", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(out))
}

func getK8sClusterInfo() map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get cluster info
	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	clusterOut, _ := cmd.CombinedOutput()

	// Get current context
	cmd2 := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	contextOut, _ := cmd2.CombinedOutput()

	// Get nodes count
	cmd3 := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "json")
	nodesOut, _ := cmd3.CombinedOutput()

	var nodeCount int
	if nodesOut != nil {
		var nodeData map[string]interface{}
		if err := json.Unmarshal(nodesOut, &nodeData); err == nil {
			if items, ok := nodeData["items"].([]interface{}); ok {
				nodeCount = len(items)
			}
		}
	}

	return map[string]any{
		"available":       true,
		"cluster_info":    strings.TrimSpace(string(clusterOut)),
		"current_context": strings.TrimSpace(string(contextOut)),
		"node_count":      nodeCount,
	}
}
