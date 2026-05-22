package app

import (
	"sort"
	"strings"

	"rbac-manager/internal/kube"

	rbacv1 "k8s.io/api/rbac/v1"
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

func ComputeGovernanceState(toolID, clusterID string, findings []Finding, plans []Plan) string {
	for _, p := range plans {
		if p.ToolID == toolID && p.ClusterID == clusterID && (p.Status == "planned" || p.Status == "validated") {
			return "in-progress"
		}
	}
	if len(findings) == 0 {
		return "secured"
	}
	for _, f := range findings {
		if f.Severity == "high" || f.Severity == "medium" {
			if f.RuleID != "no-high-risk-rbac" {
				return "needs-action"
			}
		}
	}
	return "secured"
}

func IsBaselineMatched(findings []Finding) bool {
	if len(findings) == 0 {
		return true
	}
	for _, f := range findings {
		if f.Severity == "high" || f.Severity == "medium" {
			if f.RuleID != "no-high-risk-rbac" {
				return false
			}
		}
	}
	return true
}

func filterBaselineRules(rules []kube.SARule, baselineRules []rbacv1.PolicyRule) []kube.SARule {
	filtered := []kube.SARule{}
	for _, sr := range rules {
		covered := false
		for _, br := range baselineRules {
			if ruleCovers(br, sr.Rule) {
				covered = true
				break
			}
		}
		if !covered {
			filtered = append(filtered, sr)
		}
	}
	return filtered
}

func CompareBaseline(currentRules []kube.SARule, templateIDs []string, templates *TemplateRegistry, params map[string]string) bool {
	if len(templateIDs) == 0 {
		return IsBaselineMatched(nil)
	}
	clusterRules := []kube.SARule{}
	for _, sr := range currentRules {
		if sr.Binding.Kind == "ClusterRoleBinding" || (sr.Binding.Kind == "RoleBinding" && sr.Binding.RoleKind == "ClusterRole") {
			clusterRules = append(clusterRules, sr)
		}
	}
	for _, sr := range clusterRules {
		if sr.Binding.RoleKind == "ClusterRole" && sr.Binding.RoleName == "cluster-admin" {
			return false
		}
	}
	baselineRules := extractBaselineRules(templateIDs, templates, params)
	if len(baselineRules) == 0 {
		return true
	}
	currentNormalized := normalizeRulesFromSA(clusterRules)
	baselineNormalized := normalizeRules(baselineRules)
	subsetMatch := rulesSubsetMatch(currentNormalized, baselineNormalized)
	noExtra := rulesNoExtra(currentNormalized, baselineNormalized)
	return subsetMatch && noExtra
}

func rulesNoExtra(current, baseline []rbacv1.PolicyRule) bool {
	for _, c := range current {
		if hasAny(c.Verbs, "*") && !hasAnyInBaseline(c, baseline) {
			return false
		}
		if hasAny(c.Resources, "*") && hasAny(c.Verbs, "create", "update", "patch", "delete", "deletecollection") && !hasAnyInBaseline(c, baseline) {
			return false
		}
	}
	return true
}

func hasAnyInBaseline(rule rbacv1.PolicyRule, baseline []rbacv1.PolicyRule) bool {
	for _, b := range baseline {
		if ruleCovers(b, rule) {
			return true
		}
	}
	return false
}

func extractBaselineRules(templateIDs []string, templates *TemplateRegistry, params map[string]string) []rbacv1.PolicyRule {
	if templates == nil || len(templateIDs) == 0 {
		return nil
	}
	var allRules []rbacv1.PolicyRule
	for _, tid := range templateIDs {
		tmpl, ok := templates.Get(tid)
		if !ok {
			continue
		}
		for _, res := range tmpl.Resources {
			if res.Kind == "ClusterRole" || res.Kind == "Role" {
				rendered, _, err := templates.Render(tid, params)
				if err != nil {
					continue
				}
				docs, err := kube.DecodeYAML(rendered)
				if err != nil {
					continue
				}
				for _, doc := range docs {
					if doc.GetKind() == res.Kind {
						obj := doc.Object
						if ruleList, ok := obj["rules"].([]interface{}); ok {
							for _, r := range ruleList {
								if ruleMap, ok := r.(map[string]interface{}); ok {
									allRules = append(allRules, rbacv1.PolicyRule{
										APIGroups:       toStringSlice(ruleMap["apiGroups"]),
										Resources:       toStringSlice(ruleMap["resources"]),
										Verbs:           toStringSlice(ruleMap["verbs"]),
										NonResourceURLs: toStringSlice(ruleMap["nonResourceURLs"]),
									})
								}
							}
						}
					}
				}
			}
		}
	}
	return allRules
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		out := []string{}
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func normalizeRulesFromSA(rules []kube.SARule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(rules))
	for i, sr := range rules {
		out[i] = sr.Rule
	}
	return normalizeRules(out)
}

func normalizeRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(rules))
	for i, r := range rules {
		rule := r
		sort.Strings(rule.APIGroups)
		sort.Strings(rule.Resources)
		sort.Strings(rule.Verbs)
		sort.Strings(rule.NonResourceURLs)
		out[i] = rule
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i].Verbs, ",") < strings.Join(out[j].Verbs, ",")
	})
	return out
}

func rulesSubsetMatch(current, baseline []rbacv1.PolicyRule) bool {
	for _, b := range baseline {
		found := false
		for _, c := range current {
			if ruleCovers(c, b) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func ruleCovers(actual, required rbacv1.PolicyRule) bool {
	if !verbsCover(actual.Verbs, required.Verbs) {
		return false
	}
	if !resourcesCover(actual.Resources, required.Resources) {
		return false
	}
	if !apiGroupsCover(actual.APIGroups, required.APIGroups) {
		return false
	}
	if len(required.NonResourceURLs) > 0 {
		if !nonResourceURLsCover(actual.NonResourceURLs, required.NonResourceURLs) {
			return false
		}
	}
	return true
}

func verbsCover(actual, required []string) bool {
	if contains(actual, "*") {
		return true
	}
	for _, rv := range required {
		if !contains(actual, rv) {
			return false
		}
	}
	return true
}

func resourcesCover(actual, required []string) bool {
	if contains(actual, "*") {
		return true
	}
	for _, rr := range required {
		if !contains(actual, rr) {
			return false
		}
	}
	return true
}

func apiGroupsCover(actual, required []string) bool {
	if contains(actual, "*") {
		return true
	}
	for _, rg := range required {
		if !contains(actual, rg) {
			return false
		}
	}
	return true
}

func nonResourceURLsCover(actual, required []string) bool {
	if contains(actual, "*") {
		return true
	}
	for _, ru := range required {
		if !contains(actual, ru) {
			return false
		}
	}
	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
