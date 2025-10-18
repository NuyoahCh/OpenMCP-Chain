package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 预定义的身份认证错误。
var (
	ErrDisabled           = errors.New("authentication disabled")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnsupportedGrant   = errors.New("unsupported grant type")
	ErrInvalidToken       = errors.New("invalid token")
	ErrMissingToken       = errors.New("missing bearer token")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrSubjectRevoked     = errors.New("subject is disabled")
)

// Store 定义身份认证存储的接口。
type Store interface {
	FindUserByUsername(ctx context.Context, username string) (*User, error)
	LoadSubject(ctx context.Context, userID int64) (*Subject, error)
}

// SeedWriter 定义应用种子数据以初始化存储的接口。
type SeedWriter interface {
	ApplySeed(ctx context.Context, seed Seed) error
}

// User 表示存储中的用户记录。
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Disabled     bool
}

// Subject 表示经过身份验证的主体信息。
type Subject struct {
	ID          int64
	Username    string
	Roles       []string
	Permissions []string
	Disabled    bool

	permissionsSet map[string]struct{}
}

// normalise 初始化 Subject 的内部缓存。
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

// Normalise 对外暴露的规范化方法。
func (s *Subject) Normalise() {
	s.normalise()
}

// HasPermission 检查主体是否具有指定的权限。
func (s *Subject) HasPermission(permission string) bool {
	if s == nil {
		return false
	}
	s.normalise()
	_, ok := s.permissionsSet[strings.ToLower(strings.TrimSpace(permission))]
	return ok
}

// Authorize 验证主体是否具有所有指定的权限。
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

// Clone 创建并返回主体的深拷贝，包括角色和权限切片的副本，以防止外部修改
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

// TokenRequest 是 OAuth 令牌请求的结构。
type TokenRequest struct {
	GrantType string   `json:"grant_type"`
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	Scope     []string `json:"scope"`
}

// TokenPair 表示一对访问令牌和刷新令牌。
type TokenPair struct {
	AccessToken      string   `json:"access_token"`
	ExpiresIn        int64    `json:"expires_in"`
	RefreshToken     string   `json:"refresh_token,omitempty"`
	RefreshExpiresIn int64    `json:"refresh_expires_in,omitempty"`
	TokenType        string   `json:"token_type"`
	Subject          *Subject `json:"-"`
	GrantedScopes    []string `json:"scope,omitempty"`
}

// / Config 定义身份认证服务的配置参数。
type Config struct {
	Mode  Mode
	JWT   JWTOptions
	OAuth OAuthOptions
	Seeds []Seed
}

// Mode 定义身份认证服务的工作模式。
type Mode string

// 支持的身份认证模式。
const (
	ModeDisabled Mode = "disabled"
	ModeJWT      Mode = "jwt"
	ModeOAuth    Mode = "oauth"
)

// JWTOptions 是 JWT 身份认证的配置选项。
type JWTOptions struct {
	Secret     string
	Issuer     string
	Audience   []string
	AccessTTL  int64
	RefreshTTL int64
}

// OAuthOptions 是 OAuth 身份认证的配置选项。
type OAuthOptions struct {
	TokenURL         string
	IntrospectionURL string
	ClientID         string
	ClientSecret     string
	Scopes           []string
	TimeoutSeconds   int
	UsernameClaim    string
}

// Seed 定义用于初始化身份认证存储的种子数据结构。
type Seed struct {
	Username    string
	Password    string
	Roles       []string
	Permissions []string
	Disabled    bool
}
