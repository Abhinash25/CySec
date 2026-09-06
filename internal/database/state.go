// Package database provides SQLite-backed installed tool state tracking.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// InstalledTool represents a tool that has been installed by CySec.env.
type InstalledTool struct {
	ID             string
	Name           string
	Version        string
	InstallPath    string
	ShimPath       string
	EntryCommand   string
	Adapter        string
	InstalledAt    time.Time
	UpdatedAt      time.Time
	Checksum       string
	HealthStatus   string // "healthy", "broken", "unknown"
	InstalledSize  int64  // bytes
	Status         string // "PENDING", "DOWNLOADING", "VERIFYING", "INSTALLING", "CONFIGURING", "HEALTH_CHECKING", "INSTALLED", "FAILED", "ROLLING_BACK", "ROLLED_BACK"
	Ownership      string // "CYSEC_MANAGED" or "SYSTEM_MANAGED"
}

// StateDB manages the installed tool database.
type StateDB struct {
	db     *sql.DB
	dbPath string
}

// Open opens (or creates) the state database.
func Open(dbDir string) (*StateDB, error) {
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "state.db")
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &StateDB{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// migrate creates the schema if it doesn't exist.
func (s *StateDB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS installed_tools (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		version        TEXT NOT NULL,
		install_path   TEXT NOT NULL,
		shim_path      TEXT NOT NULL DEFAULT '',
		entry_command  TEXT NOT NULL DEFAULT '',
		adapter        TEXT NOT NULL DEFAULT '',
		installed_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		checksum       TEXT NOT NULL DEFAULT '',
		health_status  TEXT NOT NULL DEFAULT 'unknown',
		installed_size INTEGER NOT NULL DEFAULT 0,
		status         TEXT NOT NULL DEFAULT 'INSTALLED',
		ownership      TEXT NOT NULL DEFAULT 'CYSEC_MANAGED'
	);

	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	INSERT OR IGNORE INTO metadata (key, value) VALUES ('schema_version', '1');
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Safe migration: Add status column if it doesn't exist (for existing MVP databases)
	_, err = s.db.Query("SELECT status FROM installed_tools LIMIT 1")
	if err != nil {
		_, err = s.db.Exec("ALTER TABLE installed_tools ADD COLUMN status TEXT NOT NULL DEFAULT 'INSTALLED'")
		if err != nil {
			return fmt.Errorf("failed to add status column during migration: %w", err)
		}
	}

	_, err = s.db.Query("SELECT ownership FROM installed_tools LIMIT 1")
	if err != nil {
		_, err = s.db.Exec("ALTER TABLE installed_tools ADD COLUMN ownership TEXT NOT NULL DEFAULT 'CYSEC_MANAGED'")
		if err != nil {
			return fmt.Errorf("failed to add ownership column during migration: %w", err)
		}
	}

	return nil
}

// RecordInstall inserts or updates a tool installation record.
func (s *StateDB) RecordInstall(tool InstalledTool) error {
	now := time.Now()
	
	// Default to INSTALLED if not set, to maintain backward compatibility
	if tool.Status == "" {
		tool.Status = "INSTALLED"
	}

	if tool.Ownership == "" {
		tool.Ownership = "CYSEC_MANAGED"
	}

	_, err := s.db.Exec(`
		INSERT INTO installed_tools (id, name, version, install_path, shim_path, entry_command, adapter, installed_at, updated_at, checksum, health_status, installed_size, status, ownership)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			install_path = excluded.install_path,
			shim_path = excluded.shim_path,
			entry_command = excluded.entry_command,
			adapter = excluded.adapter,
			updated_at = excluded.updated_at,
			checksum = excluded.checksum,
			health_status = excluded.health_status,
			installed_size = excluded.installed_size,
			status = excluded.status,
			ownership = excluded.ownership
	`, tool.ID, tool.Name, tool.Version, tool.InstallPath, tool.ShimPath,
		tool.EntryCommand, tool.Adapter, now, now, tool.Checksum,
		tool.HealthStatus, tool.InstalledSize, tool.Status, tool.Ownership)
	if err != nil {
		return fmt.Errorf("failed to record installation: %w", err)
	}
	return nil
}

// Remove deletes a tool installation record.
func (s *StateDB) Remove(id string) error {
	_, err := s.db.Exec("DELETE FROM installed_tools WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove tool record: %w", err)
	}
	return nil
}

// IsInstalled checks if a tool is successfully installed.
func (s *StateDB) IsInstalled(id string) bool {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM installed_tools WHERE id = ? AND status = 'INSTALLED'", id).Scan(&count)
	return count > 0
}

// Get retrieves an installed tool by ID.
func (s *StateDB) Get(id string) (*InstalledTool, error) {
	row := s.db.QueryRow("SELECT id, name, version, install_path, shim_path, entry_command, adapter, installed_at, updated_at, checksum, health_status, installed_size, status, ownership FROM installed_tools WHERE id = ?", id)

	var tool InstalledTool
	err := row.Scan(&tool.ID, &tool.Name, &tool.Version, &tool.InstallPath,
		&tool.ShimPath, &tool.EntryCommand, &tool.Adapter,
		&tool.InstalledAt, &tool.UpdatedAt, &tool.Checksum,
		&tool.HealthStatus, &tool.InstalledSize, &tool.Status, &tool.Ownership)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}
	return &tool, nil
}

// ListInstalled returns all successfully installed tools.
func (s *StateDB) ListInstalled() ([]InstalledTool, error) {
	rows, err := s.db.Query("SELECT id, name, version, install_path, shim_path, entry_command, adapter, installed_at, updated_at, checksum, health_status, installed_size, status, ownership FROM installed_tools WHERE status = 'INSTALLED' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}
	defer rows.Close()

	var tools []InstalledTool
	for rows.Next() {
		var t InstalledTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Version, &t.InstallPath,
			&t.ShimPath, &t.EntryCommand, &t.Adapter,
			&t.InstalledAt, &t.UpdatedAt, &t.Checksum,
			&t.HealthStatus, &t.InstalledSize, &t.Status, &t.Ownership); err != nil {
			return nil, fmt.Errorf("failed to scan tool row: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, nil
}

// UpdateHealth updates the health status of an installed tool.
func (s *StateDB) UpdateHealth(id, status string) error {
	_, err := s.db.Exec("UPDATE installed_tools SET health_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}

// UpdateStatus updates just the status of a tool.
func (s *StateDB) UpdateStatus(id, status string) error {
	_, err := s.db.Exec("UPDATE installed_tools SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}

// TotalInstalledSize returns the sum of all installed tool sizes (only INSTALLED status).
func (s *StateDB) TotalInstalledSize() (int64, error) {
	var total sql.NullInt64
	err := s.db.QueryRow("SELECT SUM(installed_size) FROM installed_tools WHERE status = 'INSTALLED'").Scan(&total)
	if err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

// Count returns the number of successfully installed tools.
func (s *StateDB) Count() int {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM installed_tools WHERE status = 'INSTALLED'").Scan(&count)
	return count
}

// ListStaleInstallations returns tools that are in an incomplete state.
func (s *StateDB) ListStaleInstallations() ([]InstalledTool, error) {
	rows, err := s.db.Query("SELECT id, name, version, install_path, shim_path, entry_command, adapter, installed_at, updated_at, checksum, health_status, installed_size, status, ownership FROM installed_tools WHERE status NOT IN ('INSTALLED', 'FAILED', 'ROLLED_BACK')")
	if err != nil {
		return nil, fmt.Errorf("failed to list stale tools: %w", err)
	}
	defer rows.Close()

	var tools []InstalledTool
	for rows.Next() {
		var t InstalledTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Version, &t.InstallPath,
			&t.ShimPath, &t.EntryCommand, &t.Adapter,
			&t.InstalledAt, &t.UpdatedAt, &t.Checksum,
			&t.HealthStatus, &t.InstalledSize, &t.Status, &t.Ownership); err != nil {
			return nil, fmt.Errorf("failed to scan tool row: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, nil
}
// GetAll returns all installed tools.
func (s *StateDB) GetAll() []InstalledTool {
	rows, err := s.db.Query("SELECT id, name, version, install_path, shim_path, entry_command, adapter, installed_at, updated_at, checksum, health_status, installed_size, status, ownership FROM installed_tools WHERE status = 'INSTALLED'")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var tools []InstalledTool
	for rows.Next() {
		var t InstalledTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Version, &t.InstallPath,
			&t.ShimPath, &t.EntryCommand, &t.Adapter,
			&t.InstalledAt, &t.UpdatedAt, &t.Checksum,
			&t.HealthStatus, &t.InstalledSize, &t.Status, &t.Ownership); err == nil {
			tools = append(tools, t)
		}
	}
	return tools
}

// GetMeta retrieves a metadata value.
func (s *StateDB) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM metadata WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

// SetMeta sets a metadata value.
func (s *StateDB) SetMeta(key, value string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)", key, value)
	return err
}

// Close closes the database connection.
func (s *StateDB) Close() error {
	return s.db.Close()
}
