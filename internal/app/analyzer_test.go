package app

import (
	"testing"

	"rbac-manager/internal/kube"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestAnalyzeClusterAdminBinding(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Namespace: "argocd", ServiceAccount: "argocd-application-controller"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "ClusterRoleBinding", Name: "argocd-admin", RoleKind: "ClusterRole", RoleName: "cluster-admin"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
	}})
	if !hasFinding(findings, "cluster-admin", "high") {
		t.Fatalf("expected high cluster-admin finding, got %#v", findings)
	}
	if !hasFinding(findings, "wildcard-rbac", "high") {
		t.Fatalf("expected high wildcard finding, got %#v", findings)
	}
}

func TestAnalyzeReadOnlyIsLowRisk(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Namespace: "monitoring", ServiceAccount: "prometheus"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "ClusterRoleBinding", Name: "prometheus-reader", RoleKind: "ClusterRole", RoleName: "prometheus-reader"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
	}})
	if !hasFinding(findings, "no-high-risk-rbac", "low") {
		t.Fatalf("expected low-risk summary, got %#v", findings)
	}
}

func TestAnalyzeBroadReadOnlyWildcardIsMediumRisk(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Type: "argocd", Namespace: "argocd", ServiceAccount: "argocd-application-controller"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "ClusterRoleBinding", Name: "argocd-controller-read", RoleKind: "ClusterRole", RoleName: "argocd-controller-read"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get", "list", "watch"}},
	}})
	if !hasFinding(findings, "read-only-wildcard-rbac", "medium") {
		t.Fatalf("expected medium read-only wildcard finding, got %#v", findings)
	}
	if hasFinding(findings, "wildcard-rbac", "high") {
		t.Fatalf("read-only wildcard should not be treated as high-risk wildcard, got %#v", findings)
	}
}

func TestAnalyzeArgoTenantImpersonationIsNotToolCleanupRisk(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Type: "argocd", Namespace: "argocd", ServiceAccount: "argocd-application-controller"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "RoleBinding", Namespace: "argocd", Name: "team-a-argocd-controller-impersonate", RoleKind: "Role", RoleName: "argocd-impersonate-team-a-deployer"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"serviceaccounts"}, Verbs: []string{"impersonate"}, ResourceNames: []string{"team-a-deployer"}},
	}})
	if !hasFinding(findings, "argocd-tenant-impersonation", "medium") {
		t.Fatalf("expected tenant impersonation finding, got %#v", findings)
	}
	if hasFinding(findings, "privilege-escalation", "high") {
		t.Fatalf("tenant impersonation should not be tool cleanup risk, got %#v", findings)
	}
	if refs := cleanupBindingRefs(findings); len(refs) != 0 {
		t.Fatalf("tenant impersonation should not be a cleanup candidate, got %#v", refs)
	}
}

func hasFinding(findings []Finding, ruleID, severity string) bool {
	for _, f := range findings {
		if f.RuleID == ruleID && f.Severity == severity {
			return true
		}
	}
	return false
}
