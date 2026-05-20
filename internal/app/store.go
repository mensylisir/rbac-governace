package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Store struct {
	mu        sync.RWMutex
	path      string
	tenants   map[string]Tenant
	users     map[string]User
	profiles  map[string]ToolProfile
	templates map[string]Template
	clusters  map[string]Cluster
	tools     map[string]ToolInstance
	plans     map[string]Plan
	audit     []AuditEvent
}

func NewStore() *Store {
	path := os.Getenv("DATA_FILE")
	if path == "" {
		path = "data/state.json"
	}
	s := &Store{
		path:      path,
		tenants:   map[string]Tenant{},
		users:     map[string]User{},
		profiles:  map[string]ToolProfile{},
		templates: map[string]Template{},
		clusters:  map[string]Cluster{},
		tools:     map[string]ToolInstance{},
		plans:     map[string]Plan{},
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		log.Printf("load state: %v", err)
	}
	s.seedDefaults()
	return s
}

func (s *Store) seedDefaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tenants) == 0 {
		s.tenants["platform"] = Tenant{ID: "platform", Name: "Platform", ClusterIDs: []string{"*"}, Namespaces: []string{"*"}}
	}
	if len(s.users) == 0 {
		s.users["admin"] = User{ID: "admin", Name: "Platform Admin", Role: "platform-admin", TenantIDs: []string{"platform"}}
	}
	for _, p := range builtinToolProfiles() {
		s.profiles[p.ID] = p
	}
	s.saveLocked()
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "-" + time.Now().Format("20060102150405")
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

func (s *Store) PutCluster(c Cluster) Cluster {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = newID("cluster")
	}
	s.clusters[c.ID] = c
	s.saveLocked()
	return c
}

func (s *Store) GetCluster(id string) (Cluster, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clusters[id]
	return c, ok
}

func (s *Store) ListClusters() []Cluster {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Cluster, 0, len(s.clusters))
	for _, c := range s.clusters {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) ListTenants() []Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) PutTenant(t Tenant) Tenant {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = newID("tenant")
	}
	s.tenants[t.ID] = t
	s.saveLocked()
	return t
}

func (s *Store) GetUser(id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, false
	}
	for _, tenantID := range u.TenantIDs {
		if t, ok := s.tenants[tenantID]; ok {
			u.Tenants = append(u.Tenants, t)
		}
	}
	return u, true
}

func (s *Store) PutUser(u User) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == "" {
		u.ID = newID("user")
	}
	s.users[u.ID] = u
	s.saveLocked()
	return u
}

func (s *Store) ListToolProfiles() []ToolProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ToolProfile, 0, len(s.profiles))
	for _, p := range s.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) PutToolProfile(p ToolProfile) ToolProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = newID("profile")
	}
	s.profiles[p.ID] = p
	s.saveLocked()
	return p
}

func (s *Store) ListCustomTemplates() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Template, 0, len(s.templates))
	for _, t := range s.templates {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) PutCustomTemplate(t Template) Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = newID("template")
	}
	t.Builtin = false
	s.templates[t.ID] = t
	s.saveLocked()
	return t
}

func (s *Store) ReplaceClusterTools(clusterID string, tools []ToolInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tools {
		if t.ClusterID == clusterID {
			delete(s.tools, id)
		}
	}
	for _, t := range tools {
		if t.ID == "" {
			t.ID = newID("tool")
		}
		t.ClusterID = clusterID
		t.UpdatedAt = time.Now()
		s.tools[t.ID] = t
	}
	s.saveLocked()
}

func (s *Store) ReplaceClusterToolsForNamespaces(clusterID string, namespaces []string, tools []ToolInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tools {
		if t.ClusterID == clusterID && containsScope(namespaces, t.Namespace) {
			delete(s.tools, id)
		}
	}
	for _, t := range tools {
		if t.ID == "" {
			t.ID = newID("tool")
		}
		t.ClusterID = clusterID
		t.UpdatedAt = time.Now()
		s.tools[t.ID] = t
	}
	s.saveLocked()
}

func (s *Store) ListTools(clusterID string) []ToolInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ToolInstance{}
	for _, t := range s.tools {
		if clusterID == "" || t.ClusterID == clusterID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Name < out[j].Name
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func (s *Store) GetTool(id string) (ToolInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tools[id]
	return t, ok
}

func (s *Store) PutPlan(p Plan) Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = newID("plan")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.plans[p.ID] = p
	s.saveLocked()
	return p
}

func (s *Store) GetPlan(id string) (Plan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.plans[id]
	return p, ok
}

func (s *Store) ListPlans() []Plan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Plan, 0, len(s.plans))
	for _, p := range s.plans {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) AddAudit(e AuditEvent) AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = newID("audit")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	s.audit = append(s.audit, e)
	s.saveLocked()
	return e
}

func (s *Store) ListAudit() []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]AuditEvent(nil), s.audit...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

type storeSnapshot struct {
	Tenants   map[string]Tenant       `json:"tenants"`
	Users     map[string]User         `json:"users"`
	Profiles  map[string]ToolProfile  `json:"profiles"`
	Templates map[string]Template     `json:"templates"`
	Clusters  map[string]Cluster      `json:"clusters"`
	Tools     map[string]ToolInstance `json:"tools"`
	Plans     map[string]Plan         `json:"plans"`
	Audit     []AuditEvent            `json:"audit"`
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var snapshot storeSnapshot
	if err := json.Unmarshal(b, &snapshot); err != nil {
		return err
	}
	if snapshot.Clusters != nil {
		s.clusters = snapshot.Clusters
	}
	if snapshot.Tenants != nil {
		s.tenants = snapshot.Tenants
	}
	if snapshot.Users != nil {
		s.users = snapshot.Users
	}
	if snapshot.Profiles != nil {
		s.profiles = snapshot.Profiles
	}
	if snapshot.Templates != nil {
		s.templates = snapshot.Templates
	}
	if snapshot.Tools != nil {
		s.tools = snapshot.Tools
	}
	if snapshot.Plans != nil {
		s.plans = snapshot.Plans
	}
	if snapshot.Audit != nil {
		s.audit = snapshot.Audit
	}
	return nil
}

func (s *Store) saveLocked() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		log.Printf("create state dir: %v", err)
		return
	}
	snapshot := storeSnapshot{Tenants: s.tenants, Users: s.users, Profiles: s.profiles, Templates: s.templates, Clusters: s.clusters, Tools: s.tools, Plans: s.plans, Audit: s.audit}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Printf("marshal state: %v", err)
		return
	}
	if err := os.WriteFile(s.path, b, 0o600); err != nil {
		log.Printf("write state: %v", err)
	}
}
