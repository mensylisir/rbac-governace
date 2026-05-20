package app

import (
	"errors"
	"net/http"
)

const (
	RolePlatformAdmin = "platform-admin"
	RoleTenantAdmin   = "tenant-admin"
	RoleViewer        = "viewer"
	RoleAuditor       = "auditor"
)

func (s *Server) currentUser(r *http.Request) User {
	userID := r.Header.Get("X-User")
	if userID == "" {
		userID = "admin"
	}
	if user, ok := s.store.GetUser(userID); ok {
		return user
	}
	return User{ID: userID, Name: userID, Role: RoleViewer}
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
