package app

import (
	"rbac-manager/internal/kube"
)

// ToolPolicy defines the interface for tool-specific RBAC analysis policies.
type ToolPolicy interface {
	Analyze(tool kube.WorkloadRef, rules []kube.SARule) []Finding
}

// ArgoCDPolicy implements ToolPolicy for ArgoCD workloads.
// ArgoCD needs broad read access to function, so read-only wildcards are exempted.
type ArgoCDPolicy struct{}

// ExemptRuleIDs returns the set of rule IDs that ArgoCD should not flag.
func (p ArgoCDPolicy) ExemptRuleIDs() map[string]struct{} {
	return map[string]struct{}{
		"read-only-wildcard-rbac": {},
	}
}

func (p ArgoCDPolicy) Analyze(tool kube.WorkloadRef, rules []kube.SARule) []Finding {
	exempts := p.ExemptRuleIDs()
	findings := []Finding{}
	seen := map[string]struct{}{}

	add := func(sev, ruleID, title, desc, resource string) {
		if _, ok := exempts[ruleID]; ok {
			return
		}
		key := sev + "|" + ruleID + "|" + title
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    sev,
			RuleID:      ruleID,
			Title:       title,
			Description: desc,
			Resource:    resource,
		})
	}

	for _, sr := range rules {
		binding := bindingRefString(sr)

		if argoTenantImpersonationRule(sr) {
			continue
		}

		if sr.Binding.RoleKind == "ClusterRole" && sr.Binding.RoleName == "cluster-admin" {
			add("high", "cluster-admin", "ServiceAccount is bound to cluster-admin", "This grants unrestricted cluster access and should be replaced by a scoped template.", binding)
		}
		if hasAny(sr.Rule.Verbs, "*") {
			add("high", "wildcard-rbac", "Wildcard RBAC permission", "Wildcard verbs grant full read/write access and should be replaced by a scoped template.", binding)
		} else if (hasAny(sr.Rule.Resources, "*") || hasAny(sr.Rule.APIGroups, "*")) && readOnlyVerbs(sr.Rule.Verbs) {
			add("low", "read-only-wildcard-rbac", "Broad read-only RBAC permission", "The binding can read a broad set of resources but does not grant write or escalation verbs.", binding)
		}
		if hasAny(sr.Rule.Verbs, "create", "update", "patch", "delete", "deletecollection") && len(sr.Rule.Resources) > 0 {
			if sr.Binding.Kind == "ClusterRoleBinding" {
				add("medium", "cluster-write", "Cluster-wide write permission", "The binding grants write permissions beyond a single namespace.", binding)
			}
		}
		if hasAny(sr.Rule.Resources, "secrets") && hasAny(sr.Rule.Verbs, "get", "list", "watch", "*") {
			severity := "medium"
			if sr.Binding.Kind == "ClusterRoleBinding" {
				severity = "high"
			}
			add(severity, "secret-read", "Secret read access", "The ServiceAccount can read Kubernetes Secrets. Confirm this is required.", binding)
		}
		if hasAny(sr.Rule.Resources, "pods/exec") && hasAny(sr.Rule.Verbs, "create", "*") {
			add("high", "pod-exec", "Pod exec permission", "Pod exec can be used to access workloads and mounted credentials.", binding)
		}
		if hasAny(sr.Rule.Verbs, "bind", "escalate", "impersonate") {
			add("high", "privilege-escalation", "Privilege escalation verb", "The ServiceAccount can bind, escalate, or impersonate privileges.", binding)
		}
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    "low",
			RuleID:      "no-high-risk-rbac",
			Title:       "No high-risk RBAC detected",
			Description: "No cluster-admin, wildcard, escalation, or broad write permissions were found for this ServiceAccount.",
			Resource:    tool.Namespace + "/" + tool.ServiceAccount,
		})
	}
	return findings
}

// DefaultPolicy implements ToolPolicy with the standard RBAC analysis rules.
type DefaultPolicy struct{}

func (p DefaultPolicy) Analyze(tool kube.WorkloadRef, rules []kube.SARule) []Finding {
	findings := []Finding{}
	seen := map[string]struct{}{}

	add := func(sev, ruleID, title, desc, resource string) {
		key := sev + "|" + ruleID + "|" + title
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    sev,
			RuleID:      ruleID,
			Title:       title,
			Description: desc,
			Resource:    resource,
		})
	}

	for _, sr := range rules {
		binding := bindingRefString(sr)

		if sr.Binding.RoleKind == "ClusterRole" && sr.Binding.RoleName == "cluster-admin" {
			add("high", "cluster-admin", "ServiceAccount is bound to cluster-admin", "This grants unrestricted cluster access and should be replaced by a scoped template.", binding)
		}
		if hasAny(sr.Rule.Verbs, "*") {
			add("high", "wildcard-rbac", "Wildcard RBAC permission", "Wildcard verbs grant full read/write access and should be replaced by a scoped template.", binding)
		} else if (hasAny(sr.Rule.Resources, "*") || hasAny(sr.Rule.APIGroups, "*")) && readOnlyVerbs(sr.Rule.Verbs) {
			add("low", "read-only-wildcard-rbac", "Broad read-only RBAC permission", "The binding can read a broad set of resources but does not grant write or escalation verbs.", binding)
		}
		if hasAny(sr.Rule.Verbs, "create", "update", "patch", "delete", "deletecollection") && len(sr.Rule.Resources) > 0 {
			if sr.Binding.Kind == "ClusterRoleBinding" {
				add("medium", "cluster-write", "Cluster-wide write permission", "The binding grants write permissions beyond a single namespace.", binding)
			}
		}
		if hasAny(sr.Rule.Resources, "secrets") && hasAny(sr.Rule.Verbs, "get", "list", "watch", "*") {
			severity := "medium"
			if sr.Binding.Kind == "ClusterRoleBinding" {
				severity = "high"
			}
			add(severity, "secret-read", "Secret read access", "The ServiceAccount can read Kubernetes Secrets. Confirm this is required.", binding)
		}
		if hasAny(sr.Rule.Resources, "pods/exec") && hasAny(sr.Rule.Verbs, "create", "*") {
			add("high", "pod-exec", "Pod exec permission", "Pod exec can be used to access workloads and mounted credentials.", binding)
		}
		if hasAny(sr.Rule.Verbs, "bind", "escalate", "impersonate") {
			add("high", "privilege-escalation", "Privilege escalation verb", "The ServiceAccount can bind, escalate, or impersonate privileges.", binding)
		}
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    "low",
			RuleID:      "no-high-risk-rbac",
			Title:       "No high-risk RBAC detected",
			Description: "No cluster-admin, wildcard, escalation, or broad write permissions were found for this ServiceAccount.",
			Resource:    tool.Namespace + "/" + tool.ServiceAccount,
		})
	}
	return findings
}

// GetPolicy returns the appropriate ToolPolicy for the given tool type.
// Falls back to DefaultPolicy if no specific policy is registered.
func GetPolicy(toolType string) ToolPolicy {
	switch toolType {
	case "argocd":
		return ArgoCDPolicy{}
	default:
		return DefaultPolicy{}
	}
}

// bindingRefString formats a SARule binding reference for display.
func bindingRefString(sr kube.SARule) string {
	binding := sr.Binding.Name
	if sr.Binding.Kind != "" {
		binding = sr.Binding.Kind + "/" + sr.Binding.Name
	}
	if sr.Binding.Namespace != "" {
		binding = sr.Binding.Namespace + "/" + binding
	}
	return binding
}
