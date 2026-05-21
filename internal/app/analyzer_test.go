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

func TestAnalyzeNonWildcardReadOnlyIsNoRisk(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Namespace: "monitoring", ServiceAccount: "prometheus"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "ClusterRoleBinding", Name: "prometheus-reader", RoleKind: "ClusterRole", RoleName: "prometheus-reader"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
	}})
	if !hasFinding(findings, "no-high-risk-rbac", "low") {
		t.Fatalf("expected low-risk summary, got %#v", findings)
	}
}

func TestAnalyzeBroadReadOnlyWildcardIsLowRisk(t *testing.T) {
	findings := analyzeRules(kube.WorkloadRef{Type: "argocd", Namespace: "argocd", ServiceAccount: "argocd-application-controller"}, []kube.SARule{{
		Binding: kube.BindingRef{Kind: "ClusterRoleBinding", Name: "argocd-controller-read", RoleKind: "ClusterRole", RoleName: "argocd-controller-read"},
		Rule:    rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get", "list", "watch"}},
	}})
	if !hasFinding(findings, "read-only-wildcard-rbac", "low") {
		t.Fatalf("expected low read-only wildcard finding, got %#v", findings)
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
	if hasFinding(findings, "argocd-tenant-impersonation", "medium") {
		t.Fatalf("tenant impersonation should not be added as a finding, got %#v", findings)
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

func TestIsGovernableWorkload(t *testing.T) {
	ref := kube.WorkloadRef{Type: "jenkins", Namespace: "ci", ServiceAccount: "jenkins"}

	t.Run("returns true when recommendations exist", func(t *testing.T) {
		if !isGovernableWorkload(ref, nil, []string{"jenkins-agent-manager"}) {
			t.Fatal("expected true when recommendations are present")
		}
	})

	t.Run("returns false with only low findings", func(t *testing.T) {
		findings := []Finding{{Severity: "low", RuleID: "no-high-risk-rbac"}}
		if isGovernableWorkload(ref, findings, nil) {
			t.Fatal("expected false when only low findings exist")
		}
	})

	t.Run("returns true for Argo controller fallback with only low findings", func(t *testing.T) {
		argoRef := kube.WorkloadRef{Type: "argocd", Name: "argocd-application-controller", Namespace: "argocd", ServiceAccount: "argocd-application-controller"}
		findings := []Finding{{Severity: "low", RuleID: "no-high-risk-rbac"}}
		if !isGovernableWorkload(argoRef, findings, nil) {
			t.Fatal("expected true for Argo application controller even with only low findings")
		}
	})
}

func TestRecommendedTemplates(t *testing.T) {
	t.Run("non-controller ArgoCD workloads return empty", func(t *testing.T) {
		ref := kube.WorkloadRef{Type: "argocd", Name: "argocd-server", Namespace: "argocd", ServiceAccount: "argocd-server"}
		templates := recommendedTemplates(ref)
		if len(templates) != 0 {
			t.Fatalf("expected empty recommendations for non-controller ArgoCD, got %v", templates)
		}
	})

	t.Run("application controller gets argocd-control-plane", func(t *testing.T) {
		ref := kube.WorkloadRef{Type: "argocd", Name: "argocd-application-controller", Namespace: "argocd", ServiceAccount: "argocd-application-controller"}
		templates := recommendedTemplates(ref)
		if len(templates) != 1 || templates[0] != "argocd-control-plane" {
			t.Fatalf("expected [argocd-control-plane], got %v", templates)
		}
	})
}
