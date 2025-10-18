package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Common errors returned by the authentication subsystem.
var (
	ErrDisabled           = errors.New("authentication disabled")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnsupportedGrant   = errors.New("unsupported grant type")
	ErrInvalidToken       = errors.New("invalid token")
	ErrMissingToken       = errors.New("missing bearer token")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrSubjectRevoked     = errors.New("subject is disabled")
)

// Store abstracts the persistent user catalogue used by the authentication
// service. Implementations must be safe for concurrent use.
type Store interface {
	FindUserByUsername(ctx context.Context, username string) (*User, error)
	LoadSubject(ctx context.Context, userID int64) (*Subject, error)
}

// SeedWriter is implemented by stores that can upsert seed users, roles and
// permissions for bootstrapping.
type SeedWriter interface {
	ApplySeed(ctx context.Context, seed Seed) error
}

// User represents a persisted account with credentials.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Disabled     bool
}

// Subject captures the information embedded in access tokens and passed to
// request handlers via context.
type Subject struct {
	ID          int64
	Username    string
	Roles       []string
	Permissions []string
	Disabled    bool

	permissionsSet map[string]struct{}
}

// normalise prepares the lookup set for permission checks.
func (s *Subject) normalise() {
	if s == nil {
		return
	}
	if s.permissionsSet == nil {
		s.permissionsSet = make(map[string]struct{}, len(s.Permissions))
		for _, perm := range s.Permissions {
			s.permissionsSet[strings.ToLower(strings.TrimSpace(perm))] = struct{}{}
		}
	}
}

// Normalise ensures internal caches are populated for exported use cases.
func (s *Subject) Normalise() {
	s.normalise()
}

// HasPermission reports whether the subject has the specified permission.
func (s *Subject) HasPermission(permission string) bool {
	if s == nil {
		return false
	}
	s.normalise()
	_, ok := s.permissionsSet[strings.ToLower(strings.TrimSpace(permission))]
	return ok
}

// Authorize ensures the subject has all required permissions.
func (s *Subject) Authorize(perms ...string) error {
	if s == nil {
		return ErrInvalidToken
	}
	if s.Disabled {
		return ErrSubjectRevoked
	}
	for _, perm := range perms {
		if perm == "" {
			continue
		}
		if !s.HasPermission(perm) {
			return fmt.Errorf("%w: missing %s", ErrPermissionDenied, perm)
		}
	}
	return nil
}

// Clone creates a shallow copy of the subject suitable for embedding in
// tokens.
func (s *Subject) Clone() *Subject {
	if s == nil {
		return nil
	}
	clone := &Subject{
		ID:          s.ID,
		Username:    s.Username,
		Roles:       append([]string(nil), s.Roles...),
		Permissions: append([]string(nil), s.Permissions...),
		Disabled:    s.Disabled,
	}
	clone.normalise()
	return clone
}

// TokenRequest describes the payload accepted by the token issuance endpoint.
type TokenRequest struct {
	GrantType string   `json:"grant_type"`
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	Scope     []string `json:"scope"`
}

// TokenPair contains the issued access and refresh tokens.
type TokenPair struct {
	AccessToken      string   `json:"access_token"`
	ExpiresIn        int64    `json:"expires_in"`
	RefreshToken     string   `json:"refresh_token,omitempty"`
	RefreshExpiresIn int64    `json:"refresh_expires_in,omitempty"`
	TokenType        string   `json:"token_type"`
	Subject          *Subject `json:"-"`
	GrantedScopes    []string `json:"scope,omitempty"`
}

// Config configures the authentication service.
type Config struct {
	Mode  Mode
	JWT   JWTOptions
	OAuth OAuthOptions
	Seeds []Seed
}

// Mode enumerates the supported authentication providers.
type Mode string

const (
	ModeDisabled Mode = "disabled"
	ModeJWT      Mode = "jwt"
	ModeOAuth    Mode = "oauth"
)

// JWTOptions contains parameters for local JWT issuance.
type JWTOptions struct {
	Secret     string
	Issuer     string
	Audience   []string
	AccessTTL  int64
	RefreshTTL int64
}

// OAuthOptions contains settings for delegating auth to an OAuth2 provider.
type OAuthOptions struct {
	TokenURL         string
	IntrospectionURL string
	ClientID         string
	ClientSecret     string
	Scopes           []string
	TimeoutSeconds   int
	UsernameClaim    string
}

// Seed defines the initial accounts and permissions to bootstrap.
type Seed struct {
	Username    string
	Password    string
	Roles       []string
	Permissions []string
	Disabled    bool
}
