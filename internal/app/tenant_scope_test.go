package app

import (
	"strings"
	"testing"

	"rbac-manager/internal/kube"
)

func TestFilterWorkloadRefsForTenantUser(t *testing.T) {
	user := User{
		Role: RoleTenantAdmin,
		Tenants: []Tenant{{
			ClusterIDs: []string{"cluster-a"},
			Namespaces: []string{"team-a"},
		}},
	}
	refs := []kube.WorkloadRef{
		{Name: "jenkins", Namespace: "team-a"},
		{Name: "argocd-server", Namespace: "argocd"},
	}

	got := filterWorkloadRefsForUser(user, "cluster-a", refs)
	if len(got) != 1 || got[0].Name != "jenkins" {
		t.Fatalf("expected only team-a workload, got %#v", got)
	}
}

func TestTenantKubeconfigUsesTokenAndNamespace(t *testing.T) {
	got := tenantKubeconfig("dev", "https://127.0.0.1:6443", []byte("ca"), "team-a", "deployer", "token-value")
	for _, want := range []string{
		"server: https://127.0.0.1:6443",
		"certificate-authority-data: Y2E=",
		"namespace: team-a",
		"user: team-a-deployer",
		"token: token-value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("kubeconfig missing %q:\n%s", want, got)
		}
	}
}
