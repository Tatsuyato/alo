package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type setupStatusRequest struct {
	SetupKey string `json:"setupKey,omitempty"`
}

type setupStatusResponse struct {
	OK            bool      `json:"ok"`
	IsInitialized bool      `json:"isInitialized"`
	SetupKey      string    `json:"setupKey,omitempty"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"createdAt,omitempty"`
}

type setupConfigRequest struct {
	SetupKey                 string `json:"setupKey"`
	AdminAPIKey              string `json:"adminApiKey"`
	DeployerAPIKey           string `json:"deployerApiKey"`
	JWTSecret                string `json:"jwtSecret,omitempty"`
	DefaultNamespace         string `json:"defaultNamespace"`
	DefaultCPULimit          string `json:"defaultCpuLimit"`
	DefaultMemoryLimit       string `json:"defaultMemoryLimit"`
	DefaultServiceAccount    string `json:"defaultServiceAccount"`
	EnableInitialAdmin       bool   `json:"enableInitialAdmin"`
	InitialAdminProjectName  string `json:"initialAdminProjectName"`
	InitialAdminProjectRepo  string `json:"initialAdminProjectRepo"`
}

type setupResponse struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	AdminKey  string `json:"adminKey,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

// isInitialized checks if the system has been set up already
func isInitialized(db *sql.DB) bool {
	// Check if any project exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM projects LIMIT 1").Scan(&count)
	return err == nil && count > 0
}

// generateSetupKey creates a random setup key
func generateSetupKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// saveSetupKey writes setup key to file
func saveSetupKey(key string) error {
	return os.WriteFile(".setup_key", []byte(key), 0600)
}

// readSetupKey reads setup key from file
func readSetupKey() (string, error) {
	data, err := os.ReadFile(".setup_key")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// deleteSetupKey removes the setup key file
func deleteSetupKey() error {
	return os.Remove(".setup_key")
}

func (a *App) setupStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	isInit := isInitialized(a.db)
	if isInit {
		writeJSON(w, http.StatusOK, setupStatusResponse{
			OK:            true,
			IsInitialized: true,
			Message:       "system already initialized",
		})
		return
	}

	// System not initialized, check for setup key
	setupKey, _ := readSetupKey()
	if setupKey == "" {
		// Generate new setup key if doesn't exist
		newKey, err := generateSetupKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate setup key")
			return
		}
		setupKey = newKey
		if err := saveSetupKey(setupKey); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save setup key")
			return
		}
	}

	writeJSON(w, http.StatusOK, setupStatusResponse{
		OK:            true,
		IsInitialized: false,
		SetupKey:      setupKey,
		Message:       "system ready for initialization",
		CreatedAt:     time.Now().UTC(),
	})
}

func (a *App) setupConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req setupConfigRequest
	if err := r.ParseForm(); err == nil {
		// Try form data first
		req.SetupKey = r.FormValue("setupKey")
		req.AdminAPIKey = r.FormValue("adminApiKey")
		req.DeployerAPIKey = r.FormValue("deployerApiKey")
		req.JWTSecret = r.FormValue("jwtSecret")
	} else {
		// Fall back to JSON
		if err := parseJSONWithFallback(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Validate setup key
	savedKey, err := readSetupKey()
	if err != nil && err != os.ErrNotExist {
		writeError(w, http.StatusInternalServerError, "failed to read setup key")
		return
	}

	if isInitialized(a.db) {
		writeError(w, http.StatusBadRequest, "system already initialized")
		return
	}

	if savedKey == "" || req.SetupKey != savedKey {
		writeError(w, http.StatusUnauthorized, "invalid or expired setup key")
		return
	}

	// Validate input
	if req.AdminAPIKey == "" || req.DeployerAPIKey == "" {
		writeError(w, http.StatusBadRequest, "adminApiKey and deployerApiKey are required")
		return
	}

	if req.DefaultNamespace == "" {
		req.DefaultNamespace = defaultNamespace
	}
	if req.DefaultCPULimit == "" {
		req.DefaultCPULimit = "1000m"
	}
	if req.DefaultMemoryLimit == "" {
		req.DefaultMemoryLimit = "1024Mi"
	}
	if req.DefaultServiceAccount == "" {
		req.DefaultServiceAccount = "default"
	}

	// Update auth service with new keys
	a.auth.apiKeys[req.AdminAPIKey] = apiKeyRecord{
		Principal: Principal{
			Subject:    "admin",
			Role:       "admin",
			Namespaces: []string{"*"},
			Permissions: []Permission{
				PermExecuteCommand, PermReadStatus, PermReadLogs, PermManageProject, PermBuildDeploy,
			},
		},
	}
	a.auth.apiKeys[req.DeployerAPIKey] = apiKeyRecord{
		Principal: Principal{
			Subject:    "deployer",
			Role:       "deployer",
			Namespaces: []string{req.DefaultNamespace},
			Permissions: []Permission{
				PermExecuteCommand, PermReadStatus, PermReadLogs, PermBuildDeploy,
			},
		},
	}

	if req.JWTSecret != "" {
		a.auth.jwtSecret = req.JWTSecret
	}

	// Optional: Create initial admin project
	if req.EnableInitialAdmin && req.InitialAdminProjectName != "" {
		_, err := createProject(a.db, createProjectRequest{
			Name:           req.InitialAdminProjectName,
			Repo:           req.InitialAdminProjectRepo,
			Environment:    "dev",
			Namespace:      req.DefaultNamespace,
			CPULimit:       req.DefaultCPULimit,
			MemoryLimit:    req.DefaultMemoryLimit,
			ServiceAccount: req.DefaultServiceAccount,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, setupResponse{
				OK:        false,
				Message:   "setup completed with warnings",
				Error:     "failed to create initial project: " + err.Error(),
				AdminKey:  req.AdminAPIKey,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})
			// Delete setup key and return
			_ = deleteSetupKey()
			return
		}
	}

	// Delete setup key file
	_ = deleteSetupKey()

	writeJSON(w, http.StatusOK, setupResponse{
		OK:        true,
		Message:   "system setup completed successfully",
		AdminKey:  req.AdminAPIKey,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Helper to parse JSON with fallback to empty if error
func parseJSONWithFallback(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(v)
}

// autoInitializeSystem performs initial system setup on first run
func autoInitializeSystem(app *App) error {
	// Generate secure keys
	adminKey, err := generateSetupKey()
	if err != nil {
		return fmt.Errorf("failed to generate admin key: %w", err)
	}
	adminKey = "admin-" + adminKey[:16] // Make it recognizable

	deployerKey, err := generateSetupKey()
	if err != nil {
		return fmt.Errorf("failed to generate deployer key: %w", err)
	}
	deployerKey = "deployer-" + deployerKey[:16]

	jwtSecret, err := generateSetupKey()
	if err != nil {
		return fmt.Errorf("failed to generate jwt secret: %w", err)
	}

	// Get defaults from environment or use hardcoded
	defaultNS := getEnv("DEFAULT_NAMESPACE", defaultNamespace)
	defaultCPU := getEnv("DEFAULT_CPU_LIMIT", "1000m")
	defaultMem := getEnv("DEFAULT_MEMORY_LIMIT", "1024Mi")
	defaultSA := getEnv("DEFAULT_SERVICE_ACCOUNT", "default")

	// Update auth service with new keys
	app.auth.apiKeys[adminKey] = apiKeyRecord{
		Principal: Principal{
			Subject:    "admin",
			Role:       "admin",
			Namespaces: []string{"*"},
			Permissions: []Permission{
				PermExecuteCommand, PermReadStatus, PermReadLogs, PermManageProject, PermBuildDeploy,
			},
		},
	}
	app.auth.apiKeys[deployerKey] = apiKeyRecord{
		Principal: Principal{
			Subject:    "deployer",
			Role:       "deployer",
			Namespaces: []string{defaultNS},
			Permissions: []Permission{
				PermExecuteCommand, PermReadStatus, PermReadLogs, PermBuildDeploy,
			},
		},
	}
	app.auth.jwtSecret = jwtSecret

	// Create initial admin project if repo is specified
	repoURL := getEnv("INITIAL_REPO_URL", "")
	if repoURL != "" {
		projectName := getEnv("INITIAL_PROJECT_NAME", "initial-project")
		_, err := createProject(app.db, createProjectRequest{
			Name:           projectName,
			Repo:           repoURL,
			Environment:    "dev",
			Namespace:      defaultNS,
			CPULimit:       defaultCPU,
			MemoryLimit:    defaultMem,
			ServiceAccount: defaultSA,
		})
		if err != nil {
			return fmt.Errorf("failed to create initial project: %w", err)
		}
	}

	// Save setup credentials to file for user reference
	credentialsPath := ".setup_credentials"
	credentialsContent := fmt.Sprintf(`Auto-initialized credentials (created at startup):

ADMIN_API_KEY=%s
DEPLOYER_API_KEY=%s
JWT_SECRET=%s

These keys are active immediately. 
WARNING: Store these securely and do not commit to version control!

Setup again by removing .setup_credentials file and restarting.
`, adminKey, deployerKey, jwtSecret)

	if err := os.WriteFile(credentialsPath, []byte(credentialsContent), 0600); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Print to console
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          SYSTEM AUTO-INITIALIZED SUCCESSFULLY             ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║ Credentials saved to: .setup_credentials                   ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Admin API Key:     %s\n", padRight(adminKey, 44))
	fmt.Printf("║ Deployer API Key:  %s\n", padRight(deployerKey, 44))
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	return nil
}

func padRight(str string, length int) string {
	if len(str) >= length {
		return str[:length]
	}
	return str + strings.Repeat(" ", length-len(str))
}
