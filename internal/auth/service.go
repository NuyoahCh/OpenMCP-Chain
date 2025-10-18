package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"OpenMCP-Chain/pkg/logger"
)

// 常量定义。
const (
	tokenTypeAccess   = "access"
	tokenTypeRefresh  = "refresh"
	grantTypePassword = "password"
	jwtHeaderJSON     = `{"alg":"HS256","typ":"JWT"}`
	passwordSaltBytes = 16
)

// encodedJWTHeader 是编码后的 JWT 头部。
var encodedJWTHeader = base64.RawURLEncoding.EncodeToString([]byte(jwtHeaderJSON))

// Service 负责 HTTP 端点的身份验证和授权。
type Service struct {
	mode  Mode
	store Store
	jwt   *jwtManager
	oauth *oauthClient
	audit *slog.Logger
}

// NewService 构造身份认证服务实例。
func NewService(ctx context.Context, cfg Config, store Store) (*Service, error) {
	mode := Mode(strings.ToLower(string(cfg.Mode)))
	svc := &Service{
		mode:  mode,
		store: store,
		audit: logger.Audit(),
	}

	switch mode {
	case ModeDisabled:
		return svc, nil
	case ModeJWT:
		if store == nil {
			return nil, errors.New("jwt mode requires a user store")
		}
		if strings.TrimSpace(cfg.JWT.Secret) == "" {
			return nil, errors.New("jwt secret must be configured")
		}
		if cfg.JWT.AccessTTL <= 0 {
			cfg.JWT.AccessTTL = 3600
		}
		if cfg.JWT.RefreshTTL <= 0 {
			cfg.JWT.RefreshTTL = 86400
		}
		svc.jwt = &jwtManager{
			secret:     []byte(cfg.JWT.Secret),
			issuer:     cfg.JWT.Issuer,
			audience:   cfg.JWT.Audience,
			accessTTL:  time.Duration(cfg.JWT.AccessTTL) * time.Second,
			refreshTTL: time.Duration(cfg.JWT.RefreshTTL) * time.Second,
		}
	case ModeOAuth:
		client, err := newOAuthClient(cfg.OAuth)
		if err != nil {
			return nil, err
		}
		svc.oauth = client
	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", cfg.Mode)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if len(cfg.Seeds) > 0 && store != nil {
		if writer, ok := store.(SeedWriter); ok {
			for _, seed := range cfg.Seeds {
				if err := writer.ApplySeed(ctx, seed); err != nil {
					return nil, fmt.Errorf("apply seed %s: %w", seed.Username, err)
				}
			}
		}
	}
	return svc, nil
}

// Mode 返回当前身份认证服务的工作模式。
func (s *Service) Mode() Mode {
	if s == nil {
		return ModeDisabled
	}
	return s.mode
}

// Authenticate 根据提供的令牌请求进行身份验证，并返回相应的令牌对。
// current mode.
func (s *Service) Authenticate(ctx context.Context, req TokenRequest) (*TokenPair, error) {
	if s == nil || s.mode == ModeDisabled {
		return nil, ErrDisabled
	}
	switch s.mode {
	case ModeJWT:
		return s.authenticateJWT(ctx, req)
	case ModeOAuth:
		return s.authenticateOAuth(ctx, req)
	default:
		return nil, ErrDisabled
	}
}

// authenticateJWT 处理基于 JWT 的身份验证请求。
func (s *Service) authenticateJWT(ctx context.Context, req TokenRequest) (*TokenPair, error) {
	grant := strings.TrimSpace(strings.ToLower(req.GrantType))
	if grant == "" {
		grant = grantTypePassword
	}
	if grant != grantTypePassword {
		return nil, ErrUnsupportedGrant
	}
	if s.store == nil {
		return nil, errors.New("user store not configured")
	}
	user, err := s.store.FindUserByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.Disabled {
		return nil, ErrSubjectRevoked
	}
	if !verifyPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}
	subject, err := s.store.LoadSubject(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("load subject: %w", err)
	}
	if subject.Disabled {
		return nil, ErrSubjectRevoked
	}
	if s.jwt == nil {
		return nil, errors.New("jwt manager not initialised")
	}
	pair, err := s.jwt.Generate(subject)
	if err != nil {
		return nil, err
	}
	pair.Subject = subject.Clone()
	pair.TokenType = "Bearer"
	return pair, nil
}

// authenticateOAuth 处理基于 OAuth 的身份验证请求。
func (s *Service) authenticateOAuth(ctx context.Context, req TokenRequest) (*TokenPair, error) {
	if s.oauth == nil {
		return nil, errors.New("oauth client not configured")
	}
	return s.oauth.exchange(ctx, req)
}

// AuthenticateRequest 验证传入请求的授权头，并返回相应的主体信息。
// associated subject.
func (s *Service) AuthenticateRequest(ctx context.Context, authorization string) (*Subject, error) {
	if s == nil || s.mode == ModeDisabled {
		return nil, ErrDisabled
	}
	parts := strings.SplitN(strings.TrimSpace(authorization), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, ErrMissingToken
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return nil, ErrMissingToken
	}
	switch s.mode {
	case ModeJWT:
		return s.verifyJWT(ctx, token)
	case ModeOAuth:
		return s.verifyOAuth(ctx, token)
	default:
		return nil, ErrDisabled
	}
}

// verifyJWT 验证 JWT 令牌并返回相应的主体信息。
func (s *Service) verifyJWT(ctx context.Context, token string) (*Subject, error) {
	if s.jwt == nil {
		return nil, errors.New("jwt manager not initialised")
	}
	claims, err := s.jwt.Verify(token)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != tokenTypeAccess {
		return nil, ErrInvalidToken
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if s.store == nil {
		return nil, errors.New("user store not configured")
	}
	subject, err := s.store.LoadSubject(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load subject: %w", err)
	}
	if subject.Disabled {
		return nil, ErrSubjectRevoked
	}
	subject.normalise()
	return subject, nil
}

// verifyOAuth 验证 OAuth 令牌并返回相应的主体信息。
func (s *Service) verifyOAuth(ctx context.Context, token string) (*Subject, error) {
	if s.oauth == nil {
		return nil, errors.New("oauth client not configured")
	}
	info, err := s.oauth.introspect(ctx, token)
	if err != nil {
		return nil, err
	}
	if !info.Active {
		return nil, ErrInvalidToken
	}
	username := info.Username
	if username == "" {
		username = info.Subject
	}
	if username == "" {
		return nil, ErrInvalidToken
	}
	if s.store == nil {
		// allow external identity without local roles
		return &Subject{Username: username, Roles: info.Roles, Permissions: info.Permissions}, nil
	}
	user, err := s.store.FindUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidToken
	}
	subject, err := s.store.LoadSubject(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("load subject: %w", err)
	}
	if subject.Disabled {
		return nil, ErrSubjectRevoked
	}
	if len(info.Permissions) > 0 {
		// merge external scopes into subject permissions
		perms := make(map[string]struct{}, len(subject.Permissions)+len(info.Permissions))
		for _, p := range subject.Permissions {
			perms[p] = struct{}{}
		}
		for _, p := range info.Permissions {
			perms[p] = struct{}{}
		}
		merged := make([]string, 0, len(perms))
		for key := range perms {
			merged = append(merged, key)
		}
		subject.Permissions = merged
	}
	subject.normalise()
	return subject, nil
}

// jwtManager 负责 JWT 令牌的签名和验证。
type jwtManager struct {
	secret     []byte
	issuer     string
	audience   []string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// jwtClaims 定义 JWT 令牌的声明结构。
type jwtClaims struct {
	Username    string   `json:"username,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	TokenType   string   `json:"type"`
	Subject     string   `json:"sub"`
	Issuer      string   `json:"iss,omitempty"`
	Audience    []string `json:"aud,omitempty"`
	IssuedAt    int64    `json:"iat,omitempty"`
	ExpiresAt   int64    `json:"exp,omitempty"`
}

// Generate 生成访问令牌和刷新令牌对。
func (m *jwtManager) Generate(subject *Subject) (*TokenPair, error) {
	if subject == nil {
		return nil, errors.New("subject required")
	}
	subject.normalise()
	now := time.Now().Unix()

	accessClaims := jwtClaims{
		Username:    subject.Username,
		Roles:       append([]string(nil), subject.Roles...),
		Permissions: append([]string(nil), subject.Permissions...),
		TokenType:   tokenTypeAccess,
		Subject:     strconv.FormatInt(subject.ID, 10),
		Issuer:      m.issuer,
		Audience:    append([]string(nil), m.audience...),
		IssuedAt:    now,
		ExpiresAt:   now + int64(m.accessTTL.Seconds()),
	}

	refreshClaims := jwtClaims{
		Username:  subject.Username,
		Roles:     append([]string(nil), subject.Roles...),
		TokenType: tokenTypeRefresh,
		Subject:   strconv.FormatInt(subject.ID, 10),
		Issuer:    m.issuer,
		Audience:  append([]string(nil), m.audience...),
		IssuedAt:  now,
		ExpiresAt: now + int64(m.refreshTTL.Seconds()),
	}

	accessToken, err := m.sign(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}
	refreshToken, err := m.sign(refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}
	return &TokenPair{
		AccessToken:      accessToken,
		ExpiresIn:        int64(m.accessTTL.Seconds()),
		RefreshToken:     refreshToken,
		RefreshExpiresIn: int64(m.refreshTTL.Seconds()),
		TokenType:        "Bearer",
	}, nil
}

// sign 使用 HMAC-SHA256 签名 JWT 令牌。
func (m *jwtManager) sign(claims jwtClaims) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := m.signature(encodedJWTHeader, payload)
	token := strings.Join([]string{encodedJWTHeader, payload, base64.RawURLEncoding.EncodeToString(signature)}, ".")
	return token, nil
}

// signature 计算 JWT 令牌的签名部分。
func (m *jwtManager) signature(header, payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(header))
	mac.Write([]byte("."))
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// Verify 验证 JWT 令牌的有效性并返回其声明。
func (m *jwtManager) Verify(token string) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	expected := m.signature(parts[0], parts[1])
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	if subtle.ConstantTimeCompare(expected, actual) != 1 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	now := time.Now().Unix()
	if claims.ExpiresAt != 0 && now > claims.ExpiresAt {
		return nil, ErrInvalidToken
	}
	if m.issuer != "" && claims.Issuer != "" && !strings.EqualFold(m.issuer, claims.Issuer) {
		return nil, ErrInvalidToken
	}
	if len(m.audience) > 0 && len(claims.Audience) > 0 {
		matched := false
		for _, expectedAud := range m.audience {
			for _, provided := range claims.Audience {
				if strings.EqualFold(strings.TrimSpace(expectedAud), strings.TrimSpace(provided)) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return nil, ErrInvalidToken
		}
	}
	return &claims, nil
}

// oauthClient 负责与 OAuth 2.0 提供者交互以进行令牌交换和验证。
type oauthClient struct {
	config OAuthOptions
	client *http.Client
}

// oauthTokenResponse 定义 OAuth 令牌响应的结构。
type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// introspectionResponse 定义 OAuth 令牌内省响应的结构。
type introspectionResponse struct {
	Active    bool   `json:"active"`
	Subject   string `json:"sub"`
	Username  string `json:"username"`
	Scope     string `json:"scope"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	ClientID  string `json:"client_id"`
	TokenType string `json:"token_type"`
}

// oauthSubject 定义通过 OAuth 内省获得的主体信息。
type oauthSubject struct {
	Active      bool
	Subject     string
	Username    string
	Roles       []string
	Permissions []string
}

// newOAuthClient 创建并配置一个新的 OAuth 客户端实例。
func newOAuthClient(cfg OAuthOptions) (*oauthClient, error) {
	if strings.TrimSpace(cfg.IntrospectionURL) == "" {
		return nil, errors.New("oauth introspection_url must be configured")
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 15
	}
	return &oauthClient{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
	}, nil
}

// exchange 处理 OAuth 令牌交换请求。
func (c *oauthClient) exchange(ctx context.Context, req TokenRequest) (*TokenPair, error) {
	if strings.TrimSpace(c.config.TokenURL) == "" {
		return nil, errors.New("oauth token_url must be configured for issuance")
	}
	form := url.Values{}
	if req.GrantType != "" {
		form.Set("grant_type", req.GrantType)
	}
	if req.Username != "" {
		form.Set("username", req.Username)
	}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	if len(req.Scope) > 0 {
		form.Set("scope", strings.Join(req.Scope, " "))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.config.ClientID != "" {
		httpReq.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth token request failed: %s", resp.Status)
	}
	var tokenResp oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode oauth token response: %w", err)
	}
	scope := tokenResp.Scope
	if scope == "" && len(req.Scope) > 0 {
		scope = strings.Join(req.Scope, " ")
	}
	var scopes []string
	if scope != "" {
		scopes = strings.Fields(scope)
	}
	return &TokenPair{
		AccessToken:   tokenResp.AccessToken,
		ExpiresIn:     tokenResp.ExpiresIn,
		RefreshToken:  tokenResp.RefreshToken,
		TokenType:     tokenResp.TokenType,
		GrantedScopes: scopes,
	}, nil
}

// introspect 验证 OAuth 令牌并返回相应的主体信息。
func (c *oauthClient) introspect(ctx context.Context, token string) (*oauthSubject, error) {
	form := url.Values{}
	form.Set("token", token)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.IntrospectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.config.ClientID != "" {
		httpReq.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth introspection failed: %s", resp.Status)
	}
	var introspect introspectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&introspect); err != nil {
		return nil, fmt.Errorf("decode introspection: %w", err)
	}
	var perms []string
	if introspect.Scope != "" {
		perms = strings.Fields(introspect.Scope)
	}
	return &oauthSubject{
		Active:      introspect.Active,
		Subject:     introspect.Subject,
		Username:    pickClaim(introspect, c.config.UsernameClaim),
		Permissions: perms,
	}, nil
}

// pickClaim 从内省响应中提取指定的声明值。
func pickClaim(resp introspectionResponse, claim string) string {
	switch strings.ToLower(claim) {
	case "username":
		return resp.Username
	case "sub", "subject":
		return resp.Subject
	case "client_id":
		return resp.ClientID
	default:
		if claim == "preferred_username" && resp.Username == "" {
			return resp.Subject
		}
		return resp.Username
	}
}

// HashPassword 对给定的密码进行哈希处理并返回哈希值。
func HashPassword(password string) (string, error) {
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password cannot be empty")
	}
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	digest := sha256.Sum256(append(salt, []byte(password)...))
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedDigest := base64.RawStdEncoding.EncodeToString(digest[:])
	return encodedSalt + ":" + encodedDigest, nil
}

// verifyPassword 验证给定的密码是否与哈希值匹配。
func verifyPassword(hashed, password string) bool {
	if hashed == "" {
		return false
	}
	parts := strings.SplitN(hashed, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	digest := sha256.Sum256(append(salt, []byte(password)...))
	return subtle.ConstantTimeCompare(expected, digest[:]) == 1
}
