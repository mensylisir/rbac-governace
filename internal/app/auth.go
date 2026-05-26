package app

import (
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: os.Getenv("KEYCLOAK_INSECURE_SKIP_VERIFY") == "true",
		},
	},
	Timeout: 10 * time.Second,
}

const (
	RolePlatformAdmin = "platform-admin"
	RoleTenantAdmin   = "tenant-admin"
	RoleViewer        = "viewer"
	RoleAuditor       = "auditor"
)

var (
	keycloakIssuer  = envOrDefault("KEYCLOAK_ISSUER", "https://keycloak-dev.rdev.tech/auth/realms/project")
	keycloakJWKSEndpoint = keycloakIssuer + "/protocol/openid-connect/certs"
)

type jwksCache struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	mu        sync.RWMutex
}

var jwks = &jwksCache{keys: make(map[string]*rsa.PublicKey)}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (c *jwksCache) get(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	pk := c.keys[kid]
	age := time.Since(c.fetchedAt)
	c.mu.RUnlock()
	if pk != nil && age < 5*time.Minute {
		return pk, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if pk := c.keys[kid]; pk != nil && time.Since(c.fetchedAt) < 5*time.Minute {
		return pk, nil
	}
	if err := c.fetch(); err != nil {
		return nil, err
	}
	pk = c.keys[kid]
	if pk == nil {
		// Fallback: if only one key exists in JWKS, try it anyway
		// (some Keycloak deployments have kid mismatch between token and JWKS)
		if len(c.keys) == 1 {
			for _, v := range c.keys {
				return v, nil
			}
		}
		return nil, fmt.Errorf("jwks: no key for kid=%s", kid)
	}
	return pk, nil
}

func (c *jwksCache) fetch() error {
	resp, err := httpClient.Get(keycloakJWKSEndpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Alg string `json:"alg"`
			Use string `json:"use"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Use != "" && k.Use != "sig" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())
		newKeys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}
	c.keys = newKeys
	c.fetchedAt = time.Now()
	return nil
}

func parseBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	prefix := "Bearer "
	if strings.HasPrefix(auth, prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

func verifyToken(tokenStr string) (*jwt.Token, jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token missing kid")
		}
		return jwks.get(kid)
	}, jwt.WithIssuer(keycloakIssuer), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, nil, errors.New("invalid token claims")
	}
	return token, claims, nil
}

func extractGroups(claims jwt.MapClaims) []string {
	var groups []string
	if g, ok := claims["groups"].([]interface{}); ok {
		for _, v := range g {
			if s, ok := v.(string); ok {
				groups = append(groups, strings.TrimPrefix(s, "/"))
			}
		}
	}
	if len(groups) == 0 {
		if ra, ok := claims["realm_access"].(map[string]interface{}); ok {
			if roles, ok := ra["roles"].([]interface{}); ok {
				for _, v := range roles {
					if s, ok := v.(string); ok {
						groups = append(groups, s)
					}
				}
			}
		}
	}
	return groups
}

func mapRole(groups []string) string {
	for _, g := range groups {
		g = strings.ToLower(g)
		if strings.Contains(g, "admin") && !strings.Contains(g, "tenant") {
			return RolePlatformAdmin
		}
		if g == "platform-admin" || g == "platform-admins" {
			return RolePlatformAdmin
		}
	}
	for _, g := range groups {
		g = strings.ToLower(g)
		if strings.Contains(g, "tenant") && strings.Contains(g, "admin") {
			return RoleTenantAdmin
		}
		if g == "tenant-admin" || g == "tenant-admins" {
			return RoleTenantAdmin
		}
	}
	for _, g := range groups {
		g = strings.ToLower(g)
		if strings.Contains(g, "auditor") {
			return RoleAuditor
		}
	}
	return RoleViewer
}

func (s *Server) currentUser(r *http.Request) User {
	tokenStr := parseBearerToken(r)
	if tokenStr == "" {
		return User{ID: "anonymous", Name: "Anonymous", Role: RoleViewer}
	}
	_, claims, err := verifyToken(tokenStr)
	if err != nil {
		return User{ID: "anonymous", Name: "Anonymous", Role: RoleViewer}
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID, _ = claims["preferred_username"].(string)
	}
	name, _ := claims["name"].(string)
	if name == "" {
		name, _ = claims["preferred_username"].(string)
	}
	if name == "" {
		name = userID
	}

	groups := extractGroups(claims)
	role := mapRole(groups)

	// Try to match existing user in store for tenant assignments
	if u, ok := s.store.GetUser(userID); ok {
		// Trust token role over stored role; keep tenant assignments
		u.Role = role
		u.Name = name
		return u
	}

	return User{ID: userID, Name: name, Role: role}
}

func canApply(user User) bool {
	return user.Role == RolePlatformAdmin || user.Role == RoleTenantAdmin
}

func canAdmin(user User) bool {
	return user.Role == RolePlatformAdmin
}

func (s *Server) authorizeCluster(user User, clusterID string) bool {
	if user.Role == RolePlatformAdmin {
		return true
	}
	for _, t := range user.Tenants {
		if containsScope(t.ClusterIDs, clusterID) {
			return true
		}
	}
	return false
}

func (s *Server) authorizeNamespace(user User, clusterID, namespace string) bool {
	if user.Role == RolePlatformAdmin {
		return true
	}
	for _, t := range user.Tenants {
		if containsScope(t.ClusterIDs, clusterID) && containsScope(t.Namespaces, namespace) {
			return true
		}
	}
	return false
}

func containsScope(values []string, target string) bool {
	for _, v := range values {
		if v == "*" || v == target {
			return true
		}
	}
	return false
}

func (s *Server) requireApply(w http.ResponseWriter, r *http.Request) (User, bool) {
	user := s.currentUser(r)
	if !canApply(user) {
		httpError(w, http.StatusForbidden, errors.New("user cannot apply changes"))
		return User{}, false
	}
	return user, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	user := s.currentUser(r)
	if !canAdmin(user) {
		httpError(w, http.StatusForbidden, errors.New("platform admin role is required"))
		return User{}, false
	}
	return user, true
}
