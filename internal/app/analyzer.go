package app

import (
	"strings"

	"rbac-manager/internal/kube"
)

func analyzeRules(tool kube.WorkloadRef, rules []kube.SARule) []Finding {
	policy := GetPolicy(tool.Type)
	return policy.Analyze(tool, rules)
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
			return []string{"argocd-control-plane"}
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
