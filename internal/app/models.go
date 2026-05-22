package app

import "time"

type Cluster struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Context             string    `json:"context"`
	Kubeconfig          string    `json:"kubeconfig,omitempty"`
	APIServer           string    `json:"apiServer"`
	Status              string    `json:"status"`
	Message             string    `json:"message"`
	RBACManagerStatus   string    `json:"rbacManagerStatus"`
	RBACDefinitionFound bool      `json:"rbacDefinitionFound"`
	LastScanAt          time.Time `json:"lastScanAt,omitempty"`
}

type Tenant struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ClusterIDs []string `json:"clusterIds"`
	Namespaces []string `json:"namespaces"`
}

type TenantCredentialRequest struct {
	ClusterID      string `json:"clusterId"`
	Namespace      string `json:"namespace"`
	ServiceAccount string `json:"serviceAccount"`
	Expiration     int64  `json:"expirationSeconds,omitempty"`
	Format         string `json:"format,omitempty"`
}

type TenantCredentialResponse struct {
	ClusterID      string `json:"clusterId"`
	Namespace      string `json:"namespace"`
	ServiceAccount string `json:"serviceAccount"`
	Expiration     int64  `json:"expirationSeconds"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
	Token          string `json:"token,omitempty"`
	Kubeconfig     string `json:"kubeconfig,omitempty"`
}

type User struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	TenantIDs []string `json:"tenantIds"`
	Tenants   []Tenant `json:"tenants,omitempty"`
}

type ToolInstance struct {
	ID                     string            `json:"id"`
	ClusterID              string            `json:"clusterId"`
	Type                   string            `json:"type"`
	Name                   string            `json:"name"`
	Namespace              string            `json:"namespace"`
	Kind                   string            `json:"kind"`
	ServiceAccount         string            `json:"serviceAccount"`
	Labels                 map[string]string `json:"labels,omitempty"`
	Findings               []Finding         `json:"findings"`
	RecommendedTemplateIDs []string          `json:"recommendedTemplateIds,omitempty"`
	GovernanceState        string            `json:"governanceState"`
	BaselineMatched        bool              `json:"baselineMatched"`
	UpdatedAt              time.Time         `json:"updatedAt,omitempty"`
}

type ToolProfile struct {
	ID                     string            `json:"id"`
	Type                   string            `json:"type"`
	Name                   string            `json:"name"`
	MatchText              string            `json:"matchText"`
	RecommendedTemplateIDs []string          `json:"recommendedTemplateIds"`
	Labels                 map[string]string `json:"labels,omitempty"`
	Builtin                bool              `json:"builtin"`
}

type Finding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	RuleID      string `json:"ruleId"`
}

type Template struct {
	ID          string             `json:"id"`
	Tool        string             `json:"tool"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Scope       string             `json:"scope"`
	RiskLevel   string             `json:"riskLevel"`
	Builtin     bool               `json:"builtin"`
	Params      []TemplateParam    `json:"params"`
	Resources   []TemplateResource `json:"resources"`
}

type TemplateParam struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

type TemplateResource struct {
	Kind     string `json:"kind"`
	Template string `json:"template"`
}

type RenderTemplateRequest struct {
	ClusterID  string            `json:"clusterId"`
	ToolID     string            `json:"toolId"`
	TemplateID string            `json:"templateId"`
	Params     map[string]string `json:"params"`
	Cleanup    bool              `json:"cleanup"`
}

type RenderTemplateResponse struct {
	YAML     string   `json:"yaml"`
	Warnings []string `json:"warnings"`
}

type Plan struct {
	ID         string             `json:"id"`
	ClusterID  string             `json:"clusterId"`
	ToolID     string             `json:"toolId"`
	TemplateID string             `json:"templateId"`
	Params     map[string]string  `json:"params"`
	YAML       string             `json:"yaml"`
	Warnings   []string           `json:"warnings"`
	Cleanup    []ResourceSnapshot `json:"cleanup,omitempty"`
	Status     string             `json:"status"`
	Validation []ValidationCheck  `json:"validation,omitempty"`
	Rollback   []ResourceSnapshot `json:"rollback,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
	AppliedAt  time.Time          `json:"appliedAt,omitempty"`
	Result     string             `json:"result,omitempty"`
}

type ValidationCheck struct {
	Allowed        bool   `json:"allowed"`
	Namespace      string `json:"namespace"`
	Verb           string `json:"verb"`
	Group          string `json:"group"`
	Resource       string `json:"resource"`
	Name           string `json:"name,omitempty"`
	Reason         string `json:"reason,omitempty"`
	ServiceAccount string `json:"serviceAccount"`
}

type ResourceSnapshot struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	YAML       string `json:"yaml,omitempty"`
	Exists     bool   `json:"exists"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	ClusterID string    `json:"clusterId,omitempty"`
	ToolID    string    `json:"toolId,omitempty"`
	PlanID    string    `json:"planId,omitempty"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}
