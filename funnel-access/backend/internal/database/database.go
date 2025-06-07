package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/models"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

const (
	dbFileName    = "funnel_controller.db"
	dbStateRecordID = 1 // We will only have one record representing the current state
)

// Store manages database operations.
type Store struct {
	db *sql.DB
	mu sync.Mutex // To protect concurrent access to the single state record
}

// NewStore creates and initializes a new Store.
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		// Default path if not specified, e.g., relative to executable or user data dir
		// For simplicity, let's place it in a 'data' subdirectory of the current working dir.
		// This should be configurable in a real application.
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		dbPath = filepath.Join(cwd, "data", dbFileName)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	log.Printf("Initializing database at: %s", dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return store, nil
}

// initSchema creates the funnel_state table if it doesn't exist and ensures a default record.
func (s *Store) initSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
    CREATE TABLE IF NOT EXISTS funnel_state (
        id INTEGER PRIMARY KEY,
        status TEXT NOT NULL,
        disable_at_timestamp DATETIME NULL,
        last_updated_timestamp DATETIME NOT NULL
    );`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create funnel_state table: %w", err)
	}

	// Ensure a single record exists to represent the funnel state
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM funnel_state WHERE id = ?", dbStateRecordID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing state record: %w", err)
	}

	if count == 0 {
		// Insert a default 'DISABLED' state if no record exists
		stmt, err := s.db.Prepare("INSERT INTO funnel_state (id, status, disable_at_timestamp, last_updated_timestamp) VALUES (?, ?, ?, ?)")
		if err != nil {
			return fmt.Errorf("failed to prepare insert statement for default state: %w", err)
		}
		defer stmt.Close()

		_, err = stmt.Exec(dbStateRecordID, models.FunnelStatusDisabled, nil, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to insert default funnel state: %w", err)
		}
		log.Println("Initialized default funnel state to DISABLED.")
	}
	return nil
}

// GetFunnelState retrieves the current funnel state from the database.
func (s *Store) GetFunnelState() (*models.FunnelState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.db.QueryRow("SELECT status, disable_at_timestamp, last_updated_timestamp FROM funnel_state WHERE id = ?", dbStateRecordID)

	var state models.FunnelState
	var disableAt sql.NullTime // Use sql.NullTime for nullable DATETIME

	err := row.Scan(&state.Status, &disableAt, &state.LastUpdatedTimestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			// This should ideally not happen if initSchema ensures a record
			log.Println("No funnel state record found, returning default DISABLED state.")
			return &models.FunnelState{
				ID:                  dbStateRecordID,
				Status:              models.FunnelStatusDisabled,
				DisableAtTimestamp:  nil,
				LastUpdatedTimestamp: time.Now().UTC(),
			}, nil
		}
		return nil, fmt.Errorf("failed to query funnel state: %w", err)
	}

	state.ID = dbStateRecordID
	if disableAt.Valid {
		state.DisableAtTimestamp = &disableAt.Time
	} else {
		state.DisableAtTimestamp = nil
	}

	return &state, nil
}

// UpdateFunnelState updates the funnel state in the database.
func (s *Store) UpdateFunnelState(status models.FunnelStatus, disableAt *time.Time) (*models.FunnelState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.db.Prepare("UPDATE funnel_state SET status = ?, disable_at_timestamp = ?, last_updated_timestamp = ? WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	var sqlDisableAt sql.NullTime
	if disableAt != nil {
		sqlDisableAt = sql.NullTime{Time: *disableAt, Valid: true}
	} else {
		sqlDisableAt = sql.NullTime{Valid: false}
	}

	res, err := stmt.Exec(status, sqlDisableAt, now, dbStateRecordID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute update statement: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("no rows updated, funnel state record might be missing (id: %d)", dbStateRecordID)
	}
	
	log.Printf("Funnel state updated: Status=%s, DisableAt=%v", status, disableAt)

	return &models.FunnelState{
		ID:                  dbStateRecordID,
		Status:              status,
		DisableAtTimestamp:  disableAt,
		LastUpdatedTimestamp: now,
	}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
