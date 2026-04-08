package main

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(10 * time.Minute)
	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  repo TEXT NOT NULL,
  environment TEXT NOT NULL,
  namespace TEXT NOT NULL,
  cpu_limit TEXT NOT NULL,
  memory_limit TEXT NOT NULL,
  service_account TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS services (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  image TEXT NOT NULL,
  replicas INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(project_id, name),
  FOREIGN KEY(project_id) REFERENCES projects(id)
);
`
	_, err := db.Exec(schema)
	return err
}

func createProject(db *sql.DB, req createProjectRequest) (Project, error) {
	if req.Namespace == "" {
		req.Namespace = defaultNamespace
	}
	if req.CPULimit == "" || req.MemoryLimit == "" {
		return Project{}, errors.New("cpuLimit and memoryLimit are required")
	}
	res, err := db.Exec(`
INSERT INTO projects(name, repo, environment, namespace, cpu_limit, memory_limit, service_account)
VALUES(?, ?, ?, ?, ?, ?, ?)`, req.Name, req.Repo, req.Environment, req.Namespace, req.CPULimit, req.MemoryLimit, req.ServiceAccount)
	if err != nil {
		return Project{}, err
	}
	id, _ := res.LastInsertId()
	return getProject(db, id)
}

func getProject(db *sql.DB, id int64) (Project, error) {
	var p Project
	var created string
	err := db.QueryRow(`
SELECT id, name, repo, environment, namespace, cpu_limit, memory_limit, service_account, created_at
FROM projects WHERE id = ?`, id).Scan(
		&p.ID, &p.Name, &p.Repo, &p.Environment, &p.Namespace, &p.CPULimit, &p.MemoryLimit, &p.ServiceAccount, &created,
	)
	if err != nil {
		return Project{}, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	return p, nil
}

func listProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(`SELECT id, name, repo, environment, namespace, cpu_limit, memory_limit, service_account, created_at FROM projects ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Repo, &p.Environment, &p.Namespace, &p.CPULimit, &p.MemoryLimit, &p.ServiceAccount, &created); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if p.CreatedAt.IsZero() {
			p.CreatedAt = time.Now().UTC()
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func createService(db *sql.DB, req createServiceRequest) (Service, error) {
	if req.Replicas <= 0 {
		req.Replicas = 1
	}
	res, err := db.Exec(`
INSERT INTO services(project_id, name, image, replicas)
VALUES(?, ?, ?, ?)`, req.ProjectID, req.Name, req.Image, req.Replicas)
	if err != nil {
		return Service{}, err
	}
	id, _ := res.LastInsertId()
	return getService(db, id)
}

func getService(db *sql.DB, id int64) (Service, error) {
	var s Service
	var created string
	err := db.QueryRow(`
SELECT id, project_id, name, image, replicas, created_at
FROM services WHERE id = ?`, id).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Image, &s.Replicas, &created)
	if err != nil {
		return Service{}, err
	}
	s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	return s, nil
}

func listServices(db *sql.DB, projectID int64) ([]Service, error) {
	rows, err := db.Query(`SELECT id, project_id, name, image, replicas, created_at FROM services WHERE project_id = ? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services := []Service{}
	for rows.Next() {
		var s Service
		var created string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Image, &s.Replicas, &created); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if s.CreatedAt.IsZero() {
			s.CreatedAt = time.Now().UTC()
		}
		services = append(services, s)
	}
	return services, rows.Err()
}
