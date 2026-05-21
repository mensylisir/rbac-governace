package app

import (
	"strings"
	"testing"
)

func TestRenderNamespaceTemplate(t *testing.T) {
	registry := NewTemplateRegistry()
	yml, warnings, err := registry.Render("jenkins-namespace-edit", map[string]string{
		"namespace":      "team-a",
		"serviceAccount": "jenkins",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	for _, want := range []string{"kind: ClusterRole", "kind: RBACDefinition", "namespace: team-a", "name: jenkins"} {
		if !strings.Contains(yml, want) {
			t.Fatalf("rendered yaml missing %q:\n%s", want, yml)
		}
	}
}

func TestRenderRequiresParams(t *testing.T) {
	registry := NewTemplateRegistry()
	_, _, err := registry.Render("jenkins-namespace-edit", map[string]string{"namespace": "team-a"})
	if err == nil {
		t.Fatal("expected missing parameter error")
	}
}

func TestHighRiskTemplateWarns(t *testing.T) {
	registry := NewTemplateRegistry()
	_, warnings, err := registry.Render("argocd-static-tenant", map[string]string{
		"namespace":              "argocd",
		"controllerServiceAccount": "argocd-application-controller",
		"serviceAccount":         "team-a-deployer",
		"targetNamespace":        "team-a",
		"sourceRepo":             "*",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected high risk warning")
	}
}
