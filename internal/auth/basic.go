package auth

import (
	"fmt"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// BasicAuthenticator validates username/password credentials against
// a list of users with bcrypt-hashed passwords from the config.
type BasicAuthenticator struct {
	users map[string]config.BasicUser
}

// NewBasicAuthenticator creates a new BasicAuthenticator from the given user list.
func NewBasicAuthenticator(users []config.BasicUser) *BasicAuthenticator {
	m := make(map[string]config.BasicUser, len(users))
	for _, u := range users {
		m[u.Username] = u
	}
	return &BasicAuthenticator{users: m}
}

// ConfigRoles returns the roles configured for the given username in the YAML
// config file, or nil if the user is not found or has no roles configured.
// These roles are used as admin overrides when no DB role assignments exist.
func (a *BasicAuthenticator) ConfigRoles(username string) []string {
	u, ok := a.users[username]
	if !ok {
		return nil
	}
	return u.Roles
}

// Authenticate checks username/password against configured users.
// Returns a UserIdentity on success or an error on failure.
// The error message is intentionally generic to prevent user enumeration.
func (a *BasicAuthenticator) Authenticate(username, password string) (*UserIdentity, error) {
	user, ok := a.users[username]
	if !ok {
		// Spend time on bcrypt to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$10$000000000000000000000uAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), []byte(password))
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return &UserIdentity{
		ProviderName: "basic",
		ProviderType: "basic",
		ExternalID:   user.Username,
		Email:        user.Username,
		Name:         user.Username,
	}, nil
}
