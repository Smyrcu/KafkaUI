package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrUserNotFound is returned by DeleteUser when the target ID does not exist.
var ErrUserNotFound = errors.New("user not found")

// UserStore persists user accounts and role assignments in a SQLite database.
type UserStore struct {
	db *sql.DB
}

// NewUserStore opens (or creates) a SQLite database at dbPath, applies schema
// migrations, and enables WAL mode and foreign-key enforcement.
// Use ":memory:" for tests.
func NewUserStore(dbPath string) (*UserStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	// SQLite does not support true concurrent writers; a single open connection
	// avoids "database is locked" errors and ensures :memory: databases share
	// the same in-process connection (required for correct test isolation).
	db.SetMaxOpenConns(1)

	// Enable WAL mode for better concurrent read performance on file databases.
	// WAL is silently a no-op for :memory: databases.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enabling WAL: %w", err)
	}

	// Enforce foreign key constraints (SQLite disables them by default).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	// Wait up to 5 seconds when the database is locked by another writer,
	// rather than immediately returning SQLITE_BUSY. This is important even
	// with MaxOpenConns(1) as the lock can be held briefly by WAL checkpoints
	// or by a second process opening the same file (e.g. during rolling restarts).
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	s := &UserStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// Close releases the database connection.
func (s *UserStore) Close() error {
	return s.db.Close()
}

// migrate creates the required tables if they do not already exist.
func (s *UserStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			provider_name TEXT NOT NULL,
			external_id   TEXT NOT NULL,
			email         TEXT NOT NULL,
			name          TEXT NOT NULL,
			avatar_url    TEXT DEFAULT '',
			orgs          TEXT DEFAULT '[]',
			teams         TEXT DEFAULT '[]',
			last_login    DATETIME NOT NULL,
			created_at    DATETIME NOT NULL,
			UNIQUE(provider_name, external_id)
		);
		CREATE TABLE IF NOT EXISTS role_assignments (
			id      TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role    TEXT NOT NULL,
			UNIQUE(user_id, role)
		);
	`)
	return err
}

// generateID generates a random 16-byte hex-encoded ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// UpsertUser inserts a new user if none exists for the provider+externalID pair,
// or updates the existing record (name, email, avatar, orgs, teams, last_login).
// Returns the current User record and a bool indicating whether it was newly created.
//
// A single atomic INSERT … ON CONFLICT … DO UPDATE is used so that concurrent
// logins never race between a SELECT and a subsequent INSERT or UPDATE.
// The created bool is derived from the RETURNING id: for a new row SQLite
// returns the id we generated; for an existing row it returns the pre-existing id.
func (s *UserStore) UpsertUser(identity *UserIdentity) (*User, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	orgsJSON, err := json.Marshal(identity.Orgs)
	if err != nil {
		return nil, false, fmt.Errorf("marshalling orgs: %w", err)
	}
	teamsJSON, err := json.Marshal(identity.Teams)
	if err != nil {
		return nil, false, fmt.Errorf("marshalling teams: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, false, err
	}

	var returnedID string
	err = s.db.QueryRow(`
		INSERT INTO users (id, provider_name, external_id, email, name, avatar_url, orgs, teams, last_login, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_name, external_id) DO UPDATE SET
			email      = excluded.email,
			name       = excluded.name,
			avatar_url = excluded.avatar_url,
			orgs       = excluded.orgs,
			teams      = excluded.teams,
			last_login = excluded.last_login
		RETURNING id`,
		id, identity.ProviderName, identity.ExternalID,
		identity.Email, identity.Name, identity.AvatarURL,
		string(orgsJSON), string(teamsJSON), now, now,
	).Scan(&returnedID)
	if err != nil {
		return nil, false, fmt.Errorf("upserting user: %w", err)
	}

	// For a new row SQLite returns the id we supplied; for an existing row it
	// returns the pre-existing id, which differs from our freshly-generated one.
	created := returnedID == id

	user, err := s.GetUser(returnedID)
	if err != nil {
		return nil, false, err
	}
	return user, created, nil
}

// GetUser returns the User with the given ID, including their roles.
// Returns an error if no such user exists.
func (s *UserStore) GetUser(id string) (*User, error) {
	u, err := s.GetUserBasic(id)
	if err != nil {
		return nil, err
	}
	u.Roles, err = s.GetRoles(id)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserBasic returns the User with the given ID without loading role assignments.
// Use this when roles will be fetched separately (e.g. via ResolveRoles) to avoid
// a redundant GetRoles query.
func (s *UserStore) GetUserBasic(id string) (*User, error) {
	row := s.db.QueryRow(`
		SELECT id, provider_name, external_id, email, name, avatar_url, orgs, teams, last_login, created_at
		FROM users WHERE id = ?`, id)

	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	return u, nil
}

// GetUserByProvider returns the User matching a provider name + external ID pair.
func (s *UserStore) GetUserByProvider(provider, externalID string) (*User, error) {
	row := s.db.QueryRow(`
		SELECT id, provider_name, external_id, email, name, avatar_url, orgs, teams, last_login, created_at
		FROM users WHERE provider_name = ? AND external_id = ?`, provider, externalID)

	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("get user by provider %s/%s: %w", provider, externalID, err)
	}

	u.Roles, err = s.GetRoles(u.ID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ListUsers returns all users including their roles.
func (s *UserStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`
		SELECT id, provider_name, external_id, email, name, avatar_url, orgs, teams, last_login, created_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	// Collect all users before closing the cursor. With MaxOpenConns(1) we
	// must not hold an open rows cursor while issuing additional queries.
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	rows.Close()

	// Now fetch roles for each user with the connection freed.
	for i := range users {
		users[i].Roles, err = s.GetRoles(users[i].ID)
		if err != nil {
			return nil, err
		}
	}

	return users, nil
}

// DeleteUser removes the user and all their role assignments (cascade).
// Returns ErrUserNotFound (via errors.Is) when no row matched the given ID.
func (s *UserStore) DeleteUser(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deleting user %s: %w", id, ErrUserNotFound)
	}
	return nil
}

// AssignRole adds a role to the user. Duplicate assignments are silently ignored.
func (s *UserStore) AssignRole(userID, role string) error {
	id, err := generateID()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO role_assignments (id, user_id, role) VALUES (?, ?, ?)`,
		id, userID, role,
	)
	if err != nil {
		return fmt.Errorf("assigning role %s to %s: %w", role, userID, err)
	}
	return nil
}

// RemoveRole removes a role from the user.
func (s *UserStore) RemoveRole(userID, role string) error {
	_, err := s.db.Exec(
		`DELETE FROM role_assignments WHERE user_id = ? AND role = ?`,
		userID, role,
	)
	if err != nil {
		return fmt.Errorf("removing role %s from %s: %w", role, userID, err)
	}
	return nil
}

// GetRoles returns the list of roles assigned to the user.
func (s *UserStore) GetRoles(userID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT role FROM role_assignments WHERE user_id = ? ORDER BY role ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting roles for %s: %w", userID, err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, fmt.Errorf("scanning role: %w", err)
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating roles: %w", err)
	}
	// Return an empty non-nil slice so callers can safely check len().
	if roles == nil {
		roles = []string{}
	}
	return roles, nil
}

// scanner abstracts *sql.Row and *sql.Rows so scanUser can handle both.
type scanner interface {
	Scan(dest ...any) error
}

// scanUser reads a user row without the roles field.
func scanUser(row scanner) (*User, error) {
	var u User
	var orgsJSON, teamsJSON string

	err := row.Scan(
		&u.ID, &u.ProviderName, &u.ExternalID,
		&u.Email, &u.Name, &u.AvatarURL,
		&orgsJSON, &teamsJSON,
		&u.LastLogin, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(orgsJSON), &u.Orgs); err != nil {
		u.Orgs = []string{}
	}
	if err := json.Unmarshal([]byte(teamsJSON), &u.Teams); err != nil {
		u.Teams = []string{}
	}
	if u.Orgs == nil {
		u.Orgs = []string{}
	}
	if u.Teams == nil {
		u.Teams = []string{}
	}

	return &u, nil
}
