package app

import (
	"fmt"
	"strings"

	"rbac-manager/internal/kube"
)

func analyzeRules(tool kube.WorkloadRef, rules []kube.SARule) []Finding {
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
		binding := fmt.Sprintf("%s/%s", sr.Binding.Kind, sr.Binding.Name)
		if sr.Binding.Namespace != "" {
			binding = sr.Binding.Namespace + "/" + binding
		}

		// Tenant impersonation binding: skip further risk checks for this single binding
		if tool.Type == "argocd" && argoTenantImpersonationRule(sr) {
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

func argoCDFindings(status kube.ArgoCDStatus, controller kube.WorkloadRef) []Finding {
	findings := []Finding{}
	if status.ApplicationControllerClusterAdmin {
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    "high",
			RuleID:      "argocd-controller-cluster-admin",
			Title:       "Argo CD controller has cluster-admin",
			Description: "Apply the Argo CD control-plane baseline first, then remove the old cluster-admin binding. Tenant ServiceAccounts are configured separately.",
			Resource:    controller.Namespace + "/" + controller.ServiceAccount,
		})
	}
	if status.Version != "" && strings.HasPrefix(strings.TrimPrefix(status.Version, "v"), "2.") && !strings.HasPrefix(strings.TrimPrefix(status.Version, "v"), "2.13") {
		findings = append(findings, Finding{
			ID:          newID("finding"),
			Severity:    "medium",
			RuleID:      "argocd-version-check",
			Title:       "Confirm Argo CD sync impersonation support",
			Description: "Sync impersonation is documented as an alpha feature since Argo CD 2.13. Verify this cluster version before applying the impersonation plan.",
			Resource:    "argocd-server:" + status.Version,
		})
	}
	return findings
}

func recommendedTemplates(ref kube.WorkloadRef) []string {
	switch strings.ToLower(ref.Type) {
	case "argocd":
		if isArgoApplicationController(ref) {
			return []string{"argocd-application-controller-view"}
		}
		return []string{}
	case "jenkins":
		return []string{"jenkins-agent-manager", "jenkins-namespace-edit"}
	case "prometheus":
		return []string{"prometheus-cluster-reader", "prometheus-namespace-reader"}
	case "loki":
		return []string{"loki-namespace-reader"}
	case "log-collector":
		return []string{"promtail-cluster-metadata-reader"}
	default:
		return []string{}
	}
}

func isGovernableWorkload(ref kube.WorkloadRef, findings []Finding, recommendations []string) bool {
	if ref.Type == "argocd" && !isArgoApplicationController(ref) {
		return false
	}
	if len(recommendations) > 0 {
		return true
	}
	for _, f := range findings {
		if f.Severity == "high" || f.Severity == "medium" {
			if f.RuleID != "no-high-risk-rbac" {
				return true
			}
		}
	}
	return isArgoApplicationController(ref)
}

func hasAny(values []string, needles ...string) bool {
	set := map[string]struct{}{}
	for _, v := range values {
		set[v] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; ok {
			return true
		}
	}
	return false
}

func readOnlyVerbs(verbs []string) bool {
	if len(verbs) == 0 {
		return false
	}
	for _, verb := range verbs {
		switch verb {
		case "get", "list", "watch":
		default:
			return false
		}
	}
	return true
}

func argoTenantImpersonationRule(sr kube.SARule) bool {
	if sr.Binding.Kind != "RoleBinding" || sr.Binding.RoleKind != "Role" {
		return false
	}
	if !hasAny(sr.Rule.Verbs, "impersonate") || !hasAny(sr.Rule.Resources, "serviceaccounts") {
		return false
	}
	return strings.HasPrefix(sr.Binding.RoleName, "argocd-impersonate-")
}
