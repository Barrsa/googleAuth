package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

// User matches the real Supabase users table schema.
// The id column is uuid DEFAULT uuid_generate_v4().
type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName,omitempty"`
	AvatarURL   string     `json:"avatarUrl,omitempty"`
	GoogleID    string     `json:"googleId,omitempty"`
	IsActive    bool       `json:"isActive"`
	IsVerified  bool       `json:"isVerified"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
}

// New connects to PostgreSQL and runs migrations.
func New(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate adds the google_id column to the existing users table.
// The users table already exists in Supabase with its own schema.
func (db *DB) migrate() error {
	_, err := db.conn.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR UNIQUE`)
	if err != nil {
		log.Printf("Migration warning (google_id): %v", err)
	}
	return nil
}

// selectCols defines the columns we SELECT from users.
const selectCols = `id, email, username, display_name, avatar_url, google_id, is_active, is_verified, created_at, updated_at, last_login`

// FindOrCreateByGoogle looks up a user by google_id. If not found, creates one.
func (db *DB) FindOrCreateByGoogle(googleID, email, displayName, avatarURL string) (*User, bool, error) {
	// Try to find by google_id first
	user, err := db.findByGoogleID(googleID)
	if err == nil {
		// Update last_login
		_, _ = db.conn.Exec("UPDATE users SET last_login = $1, updated_at = $1 WHERE id = $2", time.Now().UTC(), user.ID)
		return user, false, nil
	}

	// Try to find by email (user may have registered with password first)
	user, err = db.findByEmail(email)
	if err == nil {
		// Link Google ID to existing account
		now := time.Now().UTC()
		_, err = db.conn.Exec(
			"UPDATE users SET google_id = $1, avatar_url = $2, is_verified = TRUE, updated_at = $3, last_login = $3 WHERE id = $4",
			googleID, avatarURL, now, user.ID,
		)
		if err != nil {
			return nil, false, fmt.Errorf("link google id: %w", err)
		}
		user.GoogleID = googleID
		user.AvatarURL = avatarURL
		user.IsVerified = true
		return user, false, nil
	}

	// Create new user — generate a temp username from the email prefix
	tempUsername := generateTempUsername(email)
	// password_hash is NOT NULL; use a random placeholder that can't be used for login
	placeholderHash, err := generatePlaceholderHash()
	if err != nil {
		return nil, false, fmt.Errorf("generate placeholder hash: %w", err)
	}

	now := time.Now().UTC()
	var newID string
	err = db.conn.QueryRow(
		`INSERT INTO users (email, username, display_name, avatar_url, google_id, password_hash, is_active, is_verified, created_at, updated_at, last_login)
		 VALUES ($1, $2, $3, $4, $5, $6, TRUE, TRUE, $7, $7, $7)
		 RETURNING id`,
		email, tempUsername, displayName, avatarURL, googleID, placeholderHash, now,
	).Scan(&newID)
	if err != nil {
		return nil, false, fmt.Errorf("insert user: %w", err)
	}

	user = &User{
		ID:          newID,
		Email:       email,
		Username:    tempUsername,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		GoogleID:    googleID,
		IsActive:    true,
		IsVerified:  true,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastLogin:   &now,
	}
	return user, true, nil
}

// FindByID returns a user by their ID (uuid string).
func (db *DB) FindByID(id string) (*User, error) {
	return db.scanUser(db.conn.QueryRow(
		"SELECT "+selectCols+" FROM users WHERE id = $1", id,
	))
}

// CheckUsername returns true if the username is available.
func (db *DB) CheckUsername(username string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// UpdateUser updates mutable user fields.
func (db *DB) UpdateUser(id string, username, displayName, avatarURL *string) (*User, error) {
	sets := ""
	args := []interface{}{}
	paramIdx := 1

	if username != nil {
		sets += fmt.Sprintf("username = $%d, ", paramIdx)
		args = append(args, *username)
		paramIdx++
	}
	if displayName != nil {
		sets += fmt.Sprintf("display_name = $%d, ", paramIdx)
		args = append(args, *displayName)
		paramIdx++
	}
	if avatarURL != nil {
		sets += fmt.Sprintf("avatar_url = $%d, ", paramIdx)
		args = append(args, *avatarURL)
		paramIdx++
	}

	if len(args) == 0 {
		return db.FindByID(id)
	}

	sets += fmt.Sprintf("updated_at = $%d", paramIdx)
	args = append(args, time.Now().UTC())
	paramIdx++

	args = append(args, id)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", sets, paramIdx)

	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return db.FindByID(id)
}

// --- helpers ---

func (db *DB) findByGoogleID(googleID string) (*User, error) {
	return db.scanUser(db.conn.QueryRow(
		"SELECT "+selectCols+" FROM users WHERE google_id = $1", googleID,
	))
}

func (db *DB) findByEmail(email string) (*User, error) {
	return db.scanUser(db.conn.QueryRow(
		"SELECT "+selectCols+" FROM users WHERE email = $1", email,
	))
}

func (db *DB) scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	var username, displayName, avatarURL, googleID sql.NullString
	var lastLogin sql.NullTime
	err := row.Scan(
		&u.ID, &u.Email, &username, &displayName, &avatarURL, &googleID,
		&u.IsActive, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt, &lastLogin,
	)
	if err != nil {
		return nil, err
	}
	u.Username = username.String
	u.DisplayName = displayName.String
	u.AvatarURL = avatarURL.String
	u.GoogleID = googleID.String
	if lastLogin.Valid {
		u.LastLogin = &lastLogin.Time
	}
	return u, nil
}

// generateTempUsername creates a temporary username from the email prefix + random suffix.
func generateTempUsername(email string) string {
	prefix := strings.Split(email, "@")[0]
	// Sanitize: keep only alphanumeric and underscores
	sanitized := ""
	for _, c := range strings.ToLower(prefix) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			sanitized += string(c)
		}
	}
	if len(sanitized) < 2 {
		sanitized = "user"
	}
	if len(sanitized) > 24 {
		sanitized = sanitized[:24]
	}
	// Add random suffix for uniqueness
	b := make([]byte, 3)
	rand.Read(b)
	return sanitized + "_" + hex.EncodeToString(b)
}

// generatePlaceholderHash generates a random string to satisfy the NOT NULL password_hash constraint.
// This hash is not a valid bcrypt hash so it cannot be used for password login.
func generatePlaceholderHash() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "OAUTH_NO_PASSWORD_" + hex.EncodeToString(b), nil
}
