package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"rbac-manager/internal/kube"

	"github.com/gin-gonic/gin"
)

type Server struct {
	store     *Store
	templates *TemplateRegistry
}

func NewServer() *Server {
	store := NewStore()
	templates := NewTemplateRegistry()
	for _, t := range store.ListCustomTemplates() {
		templates.Add(t)
	}
	s := &Server{store: store, templates: templates}
	s.autoRegisterInCluster()
	return s
}

func (s *Server) Routes() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/api/health", wrap(s.handleHealth))
	r.GET("/api/me", wrap(s.handleMe))
	r.GET("/api/tenants", wrap(s.handleListTenants))
	r.POST("/api/tenants", wrap(s.handleCreateTenant))
	r.POST("/api/tenants/credentials", wrap(s.handleCreateTenantCredential))
	r.POST("/api/users", wrap(s.handleCreateUser))
	r.GET("/api/clusters", wrap(s.handleListClusters))
	r.POST("/api/clusters/import", wrap(s.handleImportCluster))
	r.POST("/api/clusters/in-cluster", wrap(s.handleImportInCluster))
	r.POST("/api/clusters/:id/test", wrapWithID(s.handleTestCluster))
	r.POST("/api/clusters/:id/scan", wrapWithID(s.handleScanCluster))
	r.GET("/api/clusters/:id/tools", wrapWithID(s.handleListClusterTools))
	r.GET("/api/templates", wrap(s.handleListTemplates))
	r.POST("/api/templates", wrap(s.handleCreateTemplate))
	r.POST("/api/templates/render", wrap(s.handleRenderTemplate))
	r.GET("/api/tool-profiles", wrap(s.handleListToolProfiles))
	r.POST("/api/tool-profiles", wrap(s.handleCreateToolProfile))
	r.GET("/api/plans", wrap(s.handleListPlans))
	r.POST("/api/plans", wrap(s.handleCreatePlan))
	r.GET("/api/plans/:id", wrapWithID(s.handleGetPlan))
	r.POST("/api/plans/:id/validate", wrapWithID(s.handleValidatePlan))
	r.POST("/api/plans/:id/apply", wrapWithID(s.handleApplyPlan))
	r.POST("/api/plans/:id/rollback", wrapWithID(s.handleRollbackPlan))
	r.GET("/api/audit-events", wrap(s.handleListAudit))
	r.GET("/api/me/permissions", wrap(s.handleMyPermissions))
	r.GET("/api/permission-requests", wrap(s.handleListPermissionRequests))
	r.POST("/api/permission-requests", wrap(s.handleCreatePermissionRequest))
	r.GET("/api/permission-requests/:id", wrapWithID(s.handleGetPermissionRequest))
	r.POST("/api/permission-requests/:id/approve", wrapWithID(s.handleApprovePermissionRequest))
	r.POST("/api/permission-requests/:id/reject", wrapWithID(s.handleRejectPermissionRequest))
	r.POST("/api/permission-requests/:id/revoke", wrapWithID(s.handleRevokePermissionRequest))

	r.StaticFile("/", filepath.Join(frontendDist(), "index.html"))
	r.StaticFile("/index.html", filepath.Join(frontendDist(), "index.html"))
	r.Static("/assets", filepath.Join(frontendDist(), "assets"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(filepath.Join(frontendDist(), "index.html"))
	})
	return r
}

func wrap(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c.Writer, c.Request)
	}
}

func wrapWithID(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.SetPathValue("id", c.Param("id"))
		handler(c.Writer, c.Request)
	}
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func frontendDist() string {
	if path := os.Getenv("FRONTEND_DIST"); path != "" {
		return path
	}
	rootPath := filepath.Join(repoRoot(), "web", "dist")
	if _, err := os.Stat(filepath.Join(rootPath, "index.html")); err == nil {
		return rootPath
	}
	if exe, err := os.Executable(); err == nil {
		exePath := filepath.Join(filepath.Dir(exe), "web", "dist")
		if _, err := os.Stat(filepath.Join(exePath, "index.html")); err == nil {
			return exePath
		}
	}
	return rootPath
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.currentUser(r))
}

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.store.ListTenants())
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var tenant Tenant
	if err := readJSON(r, &tenant); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(tenant.Name) == "" {
		httpError(w, http.StatusBadRequest, errors.New("tenant name is required"))
		return
	}
	tenant = s.store.PutTenant(tenant)
	clusterID := s.defaultClusterID()
	s.audit("tenant.create", clusterID, "", "", "success", tenant.Name)
	writeJSON(w, http.StatusCreated, tenant)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var user User
	if err := readJSON(r, &user); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(user.Name) == "" || strings.TrimSpace(user.Role) == "" {
		httpError(w, http.StatusBadRequest, errors.New("user name and role are required"))
		return
	}
	user = s.store.PutUser(user)
	s.audit("user.create", "", "", "", "success", user.Name)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	clusters := s.store.ListClusters()
	filtered := []Cluster{}
	for _, c := range clusters {
		if s.authorizeCluster(user, c.ID) {
			filtered = append(filtered, c)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

type importClusterRequest struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}

func (s *Server) handleImportCluster(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req importClusterRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Kubeconfig) == "" {
		httpError(w, http.StatusBadRequest, errors.New("name and kubeconfig are required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	client, info, err := kube.NewFromKubeconfig(req.Kubeconfig)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	status := "connected"
	message := "connected"
	if err := client.Ping(ctx); err != nil {
		status = "error"
		message = err.Error()
	}
	hasRBACManager := false
	if status == "connected" {
		hasRBACManager, _ = client.HasRBACManager(ctx)
	}
	c := s.store.PutCluster(Cluster{
		Name: req.Name, Context: info.Context, Kubeconfig: req.Kubeconfig, APIServer: info.APIServer,
		Status: status, Message: message, RBACDefinitionFound: hasRBACManager, RBACManagerStatus: rbacStatus(hasRBACManager),
	})
	s.audit("cluster.import", c.ID, "", "", status, message)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleImportInCluster(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	type request struct {
		Name string `json:"name"`
	}
	var req request
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "in-cluster"
	}
	client, info, err := kube.NewInCluster()
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	status, message := "connected", "connected"
	if err := client.Ping(ctx); err != nil {
		status, message = "error", err.Error()
	}
	hasRBACManager := false
	if status == "connected" {
		hasRBACManager, _ = client.HasRBACManager(ctx)
	}
	c := s.store.PutCluster(Cluster{Name: req.Name, Context: info.Context, Kubeconfig: "IN_CLUSTER", APIServer: info.APIServer, Status: status, Message: message, RBACDefinitionFound: hasRBACManager, RBACManagerStatus: rbacStatus(hasRBACManager)})
	s.audit("cluster.import_in_cluster", c.ID, "", "", status, message)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleTestCluster(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if !s.authorizeCluster(user, r.PathValue("id")) {
		httpError(w, http.StatusForbidden, errors.New("cluster is outside user scope"))
		return
	}
	c, client, ok := s.clientForCluster(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	status, message := "connected", "connected"
	if err := client.Ping(ctx); err != nil {
		status, message = "error", err.Error()
	}
	hasRBACManager := false
	if status == "connected" {
		hasRBACManager, _ = client.HasRBACManager(ctx)
	}
	c.Status = status
	c.Message = message
	c.RBACDefinitionFound = hasRBACManager
	c.RBACManagerStatus = rbacStatus(hasRBACManager)
	c = s.store.PutCluster(c)
	s.audit("cluster.test", c.ID, "", "", status, message)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleScanCluster(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if !s.authorizeCluster(user, r.PathValue("id")) {
		httpError(w, http.StatusForbidden, errors.New("cluster is outside user scope"))
		return
	}
	c, client, ok := s.clientForCluster(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	refs, err := client.DiscoverWorkloads(ctx)
	if err != nil {
		httpError(w, http.StatusBadGateway, err)
		return
	}
	refs = filterSystemWorkloads(refs)
	refs = filterWorkloadRefsForUser(user, r.PathValue("id"), refs)
	tools := []ToolInstance{}
	profiles := s.store.ListToolProfiles()
	for _, ref := range refs {
		if profileType, profileTemplates := matchToolProfile(ref, profiles); profileType != "" {
			ref.Type = profileType
			if len(profileTemplates) > 0 && !(ref.Type == "argocd" && !isArgoApplicationController(ref)) {
				ref.RecommendedTemplateIDs = profileTemplates
			}
		}
		rules, _ := client.RulesForServiceAccount(ctx, ref.Namespace, ref.ServiceAccount)
		recommendations := recommendedTemplates(ref)
		if len(ref.RecommendedTemplateIDs) > 0 {
			recommendations = ref.RecommendedTemplateIDs
		}
		baselineRules := extractBaselineRules(recommendations, s.templates, map[string]string{
			"namespace":              ref.Namespace,
			"serviceAccount":         ref.ServiceAccount,
			"controllerServiceAccount": ref.ServiceAccount,
		})
		for _, p := range s.store.ListPlans() {
			if p.ClusterID == c.ID && p.Status == "applied" && strings.HasPrefix(p.TemplateID, "argocd-") && p.Params["controllerServiceAccount"] == ref.ServiceAccount {
				planRules := extractBaselineRules([]string{p.TemplateID}, s.templates, p.Params)
				baselineRules = append(baselineRules, planRules...)
			}
		}
		filteredRules := filterBaselineRules(rules, baselineRules)
		findings := analyzeRules(ref, filteredRules)
		if isArgoApplicationController(ref) {
			findings = append(findings, argoCDFindings(client.ArgoCDStatus(ctx, ref.Namespace, ref.ServiceAccount), ref)...)
		}
		if !isGovernableWorkload(ref, findings, recommendations) {
			continue
		}
		toolID := c.ID + "-" + ref.Type + "-" + ref.Namespace + "-" + ref.Name
		govState := ComputeGovernanceState(toolID, c.ID, findings, s.store.ListPlans())
		baselineMatched := CompareBaseline(rules, recommendations, s.templates, map[string]string{
			"namespace":              ref.Namespace,
			"serviceAccount":         ref.ServiceAccount,
			"controllerServiceAccount": ref.ServiceAccount,
		})
		tools = append(tools, ToolInstance{
			ID: toolID, ClusterID: c.ID, Type: ref.Type, Name: ref.Name, Namespace: ref.Namespace,
			Kind: ref.Kind, ServiceAccount: ref.ServiceAccount, Labels: ref.Labels, Findings: findings, RecommendedTemplateIDs: recommendations,
			GovernanceState: govState, BaselineMatched: baselineMatched,
		})
	}
	if user.Role == RolePlatformAdmin {
		s.store.ReplaceClusterTools(c.ID, tools)
	} else {
		s.store.ReplaceClusterToolsForNamespaces(c.ID, userNamespacesForCluster(user, c.ID), tools)
	}
	c.LastScanAt = time.Now()
	s.store.PutCluster(c)
	s.audit("cluster.scan", c.ID, "", "", "success", fmt.Sprintf("discovered %d tool instances", len(tools)))
	writeJSON(w, http.StatusOK, tools)
}

func (s *Server) handleCreateTenantCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireApply(w, r)
	if !ok {
		return
	}
	var req TenantCredentialRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.ClusterID) == "" || strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.ServiceAccount) == "" {
		httpError(w, http.StatusBadRequest, errors.New("clusterId, namespace, and serviceAccount are required"))
		return
	}
	if !s.authorizeNamespace(user, req.ClusterID, req.Namespace) {
		httpError(w, http.StatusForbidden, errors.New("namespace is outside user scope"))
		return
	}
	c, client, ok := s.clientForCluster(w, r, req.ClusterID)
	if !ok {
		return
	}
	expiration := req.Expiration
	if expiration <= 0 {
		expiration = int64((8 * time.Hour).Seconds())
	}
	if expiration > int64((24 * time.Hour).Seconds()) {
		httpError(w, http.StatusBadRequest, errors.New("expirationSeconds cannot exceed 86400"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	token, err := client.CreateServiceAccountToken(ctx, req.Namespace, req.ServiceAccount, expiration)
	if err != nil {
		httpError(w, http.StatusBadGateway, err)
		return
	}
	resp := TenantCredentialResponse{
		ClusterID: req.ClusterID, Namespace: req.Namespace, ServiceAccount: req.ServiceAccount,
		Expiration: expiration, ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
	}
	if req.Format == "token" {
		resp.Token = token.Token
	} else {
		resp.Kubeconfig = tenantKubeconfig(c.Name, c.APIServer, client.ServerCAData(), req.Namespace, req.ServiceAccount, token.Token)
	}
	s.audit("tenant.credential.create", req.ClusterID, "", "", "success", req.Namespace+"/"+req.ServiceAccount)
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListClusterTools(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	clusterID := r.PathValue("id")
	if !s.authorizeCluster(user, clusterID) {
		httpError(w, http.StatusForbidden, errors.New("cluster is outside user scope"))
		return
	}
	tools := s.store.ListTools(clusterID)
	filtered := []ToolInstance{}
	for _, tool := range tools {
		if s.authorizeNamespace(user, clusterID, tool.Namespace) {
			filtered = append(filtered, tool)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.templates.List())
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var tmpl Template
	if err := readJSON(r, &tmpl); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(tmpl.ID) == "" || strings.TrimSpace(tmpl.Name) == "" || len(tmpl.Resources) == 0 {
		httpError(w, http.StatusBadRequest, errors.New("template id, name, and resources are required"))
		return
	}
	tmpl.Builtin = false
	tmpl = s.store.PutCustomTemplate(tmpl)
	s.templates.Add(tmpl)
	s.audit("template.create", "", "", "", "success", tmpl.ID)
	writeJSON(w, http.StatusCreated, tmpl)
}

func (s *Server) handleListToolProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListToolProfiles())
}

func (s *Server) handleCreateToolProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var profile ToolProfile
	if err := readJSON(r, &profile); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(profile.Type) == "" || strings.TrimSpace(profile.Name) == "" {
		httpError(w, http.StatusBadRequest, errors.New("tool profile type and name are required"))
		return
	}
	profile.Builtin = false
	profile = s.store.PutToolProfile(profile)
	s.audit("tool_profile.create", "", "", "", "success", profile.Type)
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleRenderTemplate(w http.ResponseWriter, r *http.Request) {
	var req RenderTemplateRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.resolveTemplateParams(&req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	yml, warnings, err := s.templates.Render(req.TemplateID, req.Params)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, RenderTemplateResponse{YAML: yml, Warnings: warnings})
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListPlans())
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireApply(w, r)
	if !ok {
		return
	}
	var req RenderTemplateRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if _, ok := s.store.GetCluster(req.ClusterID); !ok {
		httpError(w, http.StatusNotFound, errors.New("cluster not found"))
		return
	}
	if err := s.resolveTemplateParams(&req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if req.Params != nil && !s.authorizeNamespace(user, req.ClusterID, req.Params["namespace"]) {
		httpError(w, http.StatusForbidden, errors.New("namespace is outside user scope"))
		return
	}
	if _, ok := s.store.GetTool(req.ToolID); req.ToolID != "" && !ok {
		httpError(w, http.StatusNotFound, errors.New("tool not found"))
		return
	}
	yml, warnings, err := s.templates.Render(req.TemplateID, req.Params)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	cleanup := []ResourceSnapshot{}
	if req.Cleanup && req.ToolID != "" {
		tool, _ := s.store.GetTool(req.ToolID)
		cleanup = cleanupBindingRefs(tool.Findings)
		if len(cleanup) > 0 {
			warnings = append(warnings, fmt.Sprintf("This plan will remove %d existing risky RBAC binding(s) after the new scoped permissions are applied.", len(cleanup)))
		}
	}
	p := s.store.PutPlan(Plan{ClusterID: req.ClusterID, ToolID: req.ToolID, TemplateID: req.TemplateID, Params: req.Params, YAML: yml, Warnings: warnings, Cleanup: cleanup, Status: "planned"})
	p = s.validatePlanBestEffort(r.Context(), p)
	s.audit("plan.create", req.ClusterID, req.ToolID, p.ID, "success", "plan created")
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) resolveTemplateParams(req *RenderTemplateRequest) error {
	if req.Params == nil {
		req.Params = map[string]string{}
	}
	switch req.TemplateID {
	case "argocd-static-tenant", "argocd-dynamic-tenant":
		if strings.TrimSpace(req.Params["namespace"]) != "" && strings.TrimSpace(req.Params["controllerServiceAccount"]) != "" {
			return nil
		}
		controller, ok := s.findArgoController(req.ClusterID)
		if !ok {
			return errors.New("scan the cluster first: Argo CD application-controller was not detected")
		}
		if strings.TrimSpace(req.Params["namespace"]) == "" {
			req.Params["namespace"] = controller.Namespace
		}
		if strings.TrimSpace(req.Params["controllerServiceAccount"]) == "" {
			req.Params["controllerServiceAccount"] = controller.ServiceAccount
		}
	}
	return nil
}

func (s *Server) findArgoController(clusterID string) (ToolInstance, bool) {
	for _, tool := range s.store.ListTools(clusterID) {
		ref := kube.WorkloadRef{Type: tool.Type, Name: tool.Name, Namespace: tool.Namespace, ServiceAccount: tool.ServiceAccount, Labels: tool.Labels}
		if isArgoApplicationController(ref) {
			return tool, true
		}
	}
	return ToolInstance{}, false
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.store.GetPlan(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("plan not found"))
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleValidatePlan(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireApply(w, r)
	if !ok {
		return
	}
	p, ok := s.store.GetPlan(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("plan not found"))
		return
	}
	if !s.authorizeNamespace(user, p.ClusterID, p.Params["namespace"]) {
		httpError(w, http.StatusForbidden, errors.New("namespace is outside user scope"))
		return
	}
	p = s.validatePlanBestEffort(r.Context(), p)
	s.audit("plan.validate", p.ClusterID, p.ToolID, p.ID, "success", "plan validated")
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleApplyPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireApply(w, r)
	if !ok {
		return
	}
	p, ok := s.store.GetPlan(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("plan not found"))
		return
	}
	if p.Status == "applied" || p.Status == "rolled-back" {
		httpError(w, http.StatusConflict, errors.New("plan has already been applied or rolled back"))
		return
	}
	c, client, ok := s.clientForCluster(w, r, p.ClusterID)
	if !ok {
		return
	}
	if !s.authorizeNamespace(user, p.ClusterID, p.Params["namespace"]) {
		httpError(w, http.StatusForbidden, errors.New("namespace is outside user scope"))
		return
	}
	if !c.RBACDefinitionFound && strings.Contains(p.YAML, "kind: RBACDefinition") {
		httpError(w, http.StatusConflict, errors.New("Fairwinds RBAC Manager is not detected on this cluster"))
		return
	}
	docs, err := kube.DecodeYAML(p.YAML)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	snapshots, err := client.SnapshotObjects(ctx, docs)
	if err != nil {
		httpError(w, http.StatusBadGateway, err)
		return
	}
	if len(p.Cleanup) > 0 {
		cleanupSnapshots, err := client.SnapshotResources(ctx, toKubeSnapshots(p.Cleanup))
		if err != nil {
			httpError(w, http.StatusBadGateway, err)
			return
		}
		snapshots = append(snapshots, cleanupSnapshots...)
	}
	p.Rollback = fromKubeSnapshots(snapshots)
	p = s.store.PutPlan(p)
	if err := client.ApplyYAML(ctx, docs); err != nil {
		p.Status = "failed"
		p.Result = err.Error()
		p = s.store.PutPlan(p)
		s.audit("plan.apply", p.ClusterID, p.ToolID, p.ID, "failed", err.Error())
		httpError(w, http.StatusBadGateway, err)
		return
	}
	if len(p.Cleanup) > 0 {
		if err := client.DeleteResources(ctx, toKubeSnapshots(p.Cleanup)); err != nil {
			p.Status = "failed"
			p.Result = err.Error()
			p = s.store.PutPlan(p)
			s.audit("plan.apply", p.ClusterID, p.ToolID, p.ID, "failed", err.Error())
			httpError(w, http.StatusBadGateway, err)
			return
		}
	}
	p.Status = "applied"
	p.AppliedAt = time.Now()
	p.Result = "applied successfully"
	p = s.store.PutPlan(p)
	s.audit("plan.apply", p.ClusterID, p.ToolID, p.ID, "success", p.Result)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleRollbackPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireApply(w, r)
	if !ok {
		return
	}
	p, ok := s.store.GetPlan(r.PathValue("id"))
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("plan not found"))
		return
	}
	if !s.authorizeNamespace(user, p.ClusterID, p.Params["namespace"]) {
		httpError(w, http.StatusForbidden, errors.New("namespace is outside user scope"))
		return
	}
	_, client, ok := s.clientForCluster(w, r, p.ClusterID)
	if !ok {
		return
	}
	if len(p.Rollback) == 0 {
		httpError(w, http.StatusConflict, errors.New("plan has no rollback snapshot"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	if err := client.RestoreSnapshots(ctx, toKubeSnapshots(p.Rollback)); err != nil {
		s.audit("plan.rollback", p.ClusterID, p.ToolID, p.ID, "failed", err.Error())
		httpError(w, http.StatusBadGateway, err)
		return
	}
	p.Status = "rolled-back"
	p.Result = "rolled back successfully"
	p = s.store.PutPlan(p)
	s.audit("plan.rollback", p.ClusterID, p.ToolID, p.ID, "success", p.Result)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListAudit())
}

func (s *Server) clientForCluster(w http.ResponseWriter, r *http.Request, id string) (Cluster, *kube.Client, bool) {
	c, ok := s.store.GetCluster(id)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("cluster not found"))
		return Cluster{}, nil, false
	}
	var client *kube.Client
	var err error
	if c.Kubeconfig == "IN_CLUSTER" {
		client, _, err = kube.NewInCluster()
	} else {
		client, _, err = kube.NewFromKubeconfig(c.Kubeconfig)
	}
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return Cluster{}, nil, false
	}
	return c, client, true
}

func (s *Server) audit(action, clusterID, toolID, planID, status, message string) {
	s.store.AddAudit(AuditEvent{Action: action, ClusterID: clusterID, ToolID: toolID, PlanID: planID, Status: status, Message: message})
}

func rbacStatus(found bool) string {
	if found {
		return "installed"
	}
	return "missing"
}

var systemNamespaces = map[string]struct{}{
	"kube-system":        {},
	"local-path-storage": {},
	"rbac-manager":       {},
	"rbac-governance":    {},
}

func filterSystemWorkloads(refs []kube.WorkloadRef) []kube.WorkloadRef {
	out := make([]kube.WorkloadRef, 0, len(refs))
	for _, ref := range refs {
		if _, ok := systemNamespaces[ref.Namespace]; ok {
			continue
		}
		if ref.Name == "kube-proxy" {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func filterWorkloadRefsForUser(user User, clusterID string, refs []kube.WorkloadRef) []kube.WorkloadRef {
	if user.Role == RolePlatformAdmin {
		return refs
	}
	out := make([]kube.WorkloadRef, 0, len(refs))
	for _, ref := range refs {
		for _, tenant := range user.Tenants {
			if containsScope(tenant.ClusterIDs, clusterID) && containsScope(tenant.Namespaces, ref.Namespace) {
				out = append(out, ref)
				break
			}
		}
	}
	return out
}

func userNamespacesForCluster(user User, clusterID string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, tenant := range user.Tenants {
		if !containsScope(tenant.ClusterIDs, clusterID) {
			continue
		}
		for _, namespace := range tenant.Namespaces {
			if _, ok := seen[namespace]; ok {
				continue
			}
			seen[namespace] = struct{}{}
			out = append(out, namespace)
		}
	}
	return out
}

func tenantKubeconfig(clusterName, apiServer string, caData []byte, namespace, serviceAccount, token string) string {
	if strings.TrimSpace(clusterName) == "" {
		clusterName = "tenant-cluster"
	}
	userName := namespace + "-" + serviceAccount
	caLine := "    insecure-skip-tls-verify: true\n"
	if len(caData) > 0 {
		caLine = "    certificate-authority-data: " + base64.StdEncoding.EncodeToString(caData) + "\n"
	}
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: %s
  cluster:
    server: %s
%scontexts:
- name: %s
  context:
    cluster: %s
    namespace: %s
    user: %s
current-context: %s
users:
- name: %s
  user:
    token: %s
`, clusterName, apiServer, caLine, clusterName, clusterName, namespace, userName, clusterName, userName, token)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) defaultClusterID() string {
	clusters := s.store.ListClusters()
	if len(clusters) == 0 {
		return ""
	}
	return clusters[0].ID
}
