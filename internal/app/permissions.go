package app

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"rbac-manager/internal/kube"
)

// ---------- requests ----------

type CreatePermissionRequestRequest struct {
	RequesterID string            `json:"requesterId"`
	TemplateID  string            `json:"templateId"`
	ClusterID   string            `json:"clusterId"`
	Params      map[string]string `json:"params"`
	Reason      string            `json:"reason"`
}

func (s *Server) handleCreatePermissionRequest(w http.ResponseWriter, r *http.Request) {
	var req CreatePermissionRequestRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	if req.TemplateID == "" || req.ClusterID == "" {
		httpError(w, http.StatusBadRequest, errors.New("templateId and clusterId are required"))
		return
	}
	if _, ok := s.store.GetCluster(req.ClusterID); !ok {
		httpError(w, http.StatusNotFound, errors.New("cluster not found"))
		return
	}
	tmpl, ok := s.templates.Get(req.TemplateID)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("template not found"))
		return
	}

	requesterID := req.RequesterID
	if requesterID == "" {
		requesterID = s.currentUser(r).ID
	}

	pr := PermissionRequest{
		RequesterID: requesterID,
		TemplateID:  req.TemplateID,
		ClusterID:   req.ClusterID,
		Params:      req.Params,
		Reason:      req.Reason,
		RiskLevel:   tmpl.RiskLevel,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	// Auto-approve low-risk templates
	if tmpl.RiskLevel == "low" {
		pr.Status = "auto-approved"
		pr.ResolvedAt = time.Now()
		pr = s.store.PutPermissionRequest(pr)
		// Immediately create and apply plan
		s.autoApplyPermissionRequest(r, &pr)
		writeJSON(w, http.StatusCreated, pr)
		return
	}

	pr = s.store.PutPermissionRequest(pr)
	writeJSON(w, http.StatusCreated, pr)
}

func (s *Server) autoApplyPermissionRequest(r *http.Request, pr *PermissionRequest) {
	if pr.Status != "auto-approved" && pr.Status != "approved" {
		return
	}
	// Build a RenderTemplateRequest from the permission request
	renderReq := RenderTemplateRequest{
		ClusterID:  pr.ClusterID,
		TemplateID: pr.TemplateID,
		Params:     pr.Params,
	}
	s.resolveTemplateParams(&renderReq)
	if err := s.resolveTemplateParams(&renderReq); err != nil {
		pr.Status = "failed"
		pr.RejectReason = "auto-apply param resolution failed: " + err.Error()
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	yml, _, err := s.templates.Render(renderReq.TemplateID, renderReq.Params)
	if err != nil {
		pr.Status = "failed"
		pr.RejectReason = "auto-apply render failed: " + err.Error()
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	// Create a mock request with admin context for internal apply
	plan := s.store.PutPlan(Plan{
		ClusterID:  pr.ClusterID,
		TemplateID: pr.TemplateID,
		Params:     renderReq.Params,
		YAML:       yml,
		Status:     "planned",
		CreatedAt:  time.Now(),
	})
	pr.PlanID = plan.ID
	*pr = s.store.PutPermissionRequest(*pr)

	// Best-effort validation
	plan = s.validatePlanBestEffort(r.Context(), plan)

	c, client, ok := s.clientForClusterNoHTTP(pr.ClusterID)
	if !ok {
		pr.Status = "failed"
		pr.RejectReason = "cluster client unavailable for auto-apply"
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	if !c.RBACDefinitionFound && strings.Contains(plan.YAML, "kind: RBACDefinition") {
		pr.Status = "failed"
		pr.RejectReason = "RBAC Manager not detected on cluster"
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	docs, err := kube.DecodeYAML(yml)
	if err != nil {
		pr.Status = "failed"
		pr.RejectReason = "YAML decode failed: " + err.Error()
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	// Take snapshots for rollback before applying
	snapshots, err := client.SnapshotObjects(r.Context(), docs)
	if err != nil {
		pr.Status = "failed"
		pr.RejectReason = "snapshot failed: " + err.Error()
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	// Update plan with rollback snapshots
	if plan, ok := s.store.GetPlan(pr.PlanID); ok {
		plan.Rollback = fromKubeSnapshots(snapshots)
		_ = s.store.PutPlan(plan)
	}

	ctx := r.Context()
	if err := client.ApplyYAML(ctx, docs); err != nil {
		pr.Status = "failed"
		pr.RejectReason = "apply failed: " + err.Error()
		*pr = s.store.PutPermissionRequest(*pr)
		return
	}
	plan.Status = "applied"
	plan.AppliedAt = time.Now()
	plan.Result = "applied successfully"
	_ = s.store.PutPlan(plan)
	pr.Status = "applied"
	pr.ResolvedAt = time.Now()
	*pr = s.store.PutPermissionRequest(*pr)
}


func (s *Server) handleListPermissionRequests(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	q := r.URL.Query().Get("status")
	var out []PermissionRequest
	if canAdmin(user) {
		if q == "pending" {
			out = s.store.ListPendingPermissionRequests()
		} else {
			out = s.store.ListPermissionRequests()
		}
	} else {
		out = s.store.ListPermissionRequestsByRequester(user.ID)
		if q != "" {
			filtered := make([]PermissionRequest, 0)
			for _, pr := range out {
				if pr.Status == q {
					filtered = append(filtered, pr)
				}
			}
			out = filtered
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetPermissionRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pr, ok := s.store.GetPermissionRequest(id)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("permission request not found"))
		return
	}
	user := s.currentUser(r)
	if !canAdmin(user) && pr.RequesterID != user.ID {
		httpError(w, http.StatusForbidden, errors.New("not authorized"))
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

type ApproveRequest struct {
	ApproverID   string `json:"approverId,omitempty"`
	RejectReason string `json:"rejectReason,omitempty"`
}

func (s *Server) handleApprovePermissionRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	pr, ok := s.store.GetPermissionRequest(id)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("permission request not found"))
		return
	}
	if pr.Status != "pending" {
		httpError(w, http.StatusConflict, errors.New("request is not pending"))
		return
	}
	pr.Status = "approved"
	pr.ResolvedAt = time.Now()
	pr.ApproverID = user.ID
	pr = s.store.PutPermissionRequest(pr)

	// Auto-apply after approval
	s.autoApplyPermissionRequest(r, &pr)
	writeJSON(w, http.StatusOK, pr)
}

func (s *Server) handleRejectPermissionRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	pr, ok := s.store.GetPermissionRequest(id)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("permission request not found"))
		return
	}
	if pr.Status != "pending" {
		httpError(w, http.StatusConflict, errors.New("request is not pending"))
		return
	}
	var req ApproveRequest
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	pr.Status = "rejected"
	pr.ResolvedAt = time.Now()
	pr.ApproverID = user.ID
	pr.RejectReason = req.RejectReason
	pr = s.store.PutPermissionRequest(pr)
	writeJSON(w, http.StatusOK, pr)
}

func (s *Server) handleRevokePermissionRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	pr, ok := s.store.GetPermissionRequest(id)
	if !ok {
		httpError(w, http.StatusNotFound, errors.New("permission request not found"))
		return
	}
	if pr.Status != "applied" {
		httpError(w, http.StatusConflict, errors.New("can only revoke applied requests"))
		return
	}
	// Rollback the associated plan if exists
	if pr.PlanID != "" {
		if plan, ok := s.store.GetPlan(pr.PlanID); ok && len(plan.Rollback) > 0 {
			_, client, ok2 := s.clientForClusterNoHTTP(pr.ClusterID)
			if ok2 {
				ctx := r.Context()
				_ = client.RestoreSnapshots(ctx, toKubeSnapshots(plan.Rollback))
				plan.Status = "rolled-back"
				_ = s.store.PutPlan(plan)
			}
		}
	}
	pr.Status = "revoked"
	pr.ResolvedAt = time.Now()
	pr = s.store.PutPermissionRequest(pr)
	writeJSON(w, http.StatusOK, pr)
}

// ---------- my permissions ----------

func (s *Server) handleMyPermissions(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	requests := s.store.ListPermissionRequestsByRequester(user.ID)

	view := MyPermissionView{
		User: UserView{ID: user.ID, Name: user.Name},
	}

	for _, pr := range requests {
		view.Requests = append(view.Requests, pr)
		if pr.Status != "applied" {
			continue
		}
		tmpl, ok := s.templates.Get(pr.TemplateID)
		if !ok {
			continue
		}
		source := PermissionSource{
			TemplateID:  pr.TemplateID,
			RequestedAt: pr.CreatedAt,
			ApproverID:  pr.ApproverID,
		}
		// Extract namespace permission from params
		ns := pr.Params["namespace"]
		if ns == "" {
			ns = pr.Params["targetNamespace"]
		}
		if ns != "" {
			res := extractResourcesFromTemplate(tmpl)
			view.Namespaces = append(view.Namespaces, NamespacePermission{
				Name:      ns,
				Access:    tmpl.RiskLevel,
				Resources: res,
				Source:    source,
			})
		}
		// Extract tool permission
		if permissions := extractToolPermissions(tmpl, pr.Params); len(permissions) > 0 {
			view.Tools = append(view.Tools, ToolPermission{
				Tool:        tmpl.Tool,
				Name:        pr.Params["serviceAccount"],
				Namespace:   ns,
				Permissions: permissions,
				Source:      source,
			})
		}
	}

	writeJSON(w, http.StatusOK, view)
}

func extractResourcesFromTemplate(tmpl Template) []string {
	var resources []string
	for _, res := range tmpl.Resources {
		if res.Kind == "ClusterRole" || res.Kind == "Role" {
			// Parse YAML to extract resource list... simplified
			if strings.Contains(res.Template, "deployments") {
				resources = append(resources, "deployments")
			}
			if strings.Contains(res.Template, "services") {
				resources = append(resources, "services")
			}
			if strings.Contains(res.Template, "configmaps") {
				resources = append(resources, "configmaps")
			}
			if strings.Contains(res.Template, "secrets") {
				resources = append(resources, "secrets")
			}
			if strings.Contains(res.Template, "jobs") || strings.Contains(res.Template, "cronjobs") {
				resources = append(resources, "jobs")
			}
			if strings.Contains(res.Template, "ingresses") {
				resources = append(resources, "ingresses")
			}
			if strings.Contains(res.Template, "pods") {
				resources = append(resources, "pods")
			}
		}
	}
	return resources
}

func extractToolPermissions(tmpl Template, params map[string]string) []string {
	var perms []string
	switch tmpl.ID {
	case "argocd-static-tenant", "argocd-dynamic-tenant":
		ns := params["targetNamespace"]
		if ns == "" {
			ns = params["namespace"]
		}
		if ns != "" {
			perms = append(perms, "applications:*", "projects:get")
		}
	case "argocd-control-plane":
		perms = append(perms, "cluster:argocd-application-controller-read")
	case "prometheus-cluster-reader":
		perms = append(perms, "metrics:read", "discovery:cluster-wide")
	case "prometheus-namespace-reader":
		perms = append(perms, "metrics:read", "discovery:namespace-scoped")
	case "jenkins-agent-manager":
		perms = append(perms, "ci:manage-agents")
	case "jenkins-namespace-edit":
		perms = append(perms, "ci:deploy")
	}
	return perms
}


