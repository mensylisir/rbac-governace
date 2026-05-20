package app

import (
	"context"
	"strings"
	"time"

	"rbac-manager/internal/kube"
)

func (s *Server) validatePlanBestEffort(ctx context.Context, p Plan) Plan {
	serviceAccountNamespace := p.Params["namespace"]
	accessNamespace := p.Params["namespace"]
	serviceAccount := p.Params["serviceAccount"]
	if p.TemplateID == "argocd-tenant-sync-project" && p.Params["targetNamespace"] != "" && p.Params["serviceAccount"] != "" {
		serviceAccountNamespace = p.Params["namespace"]
		accessNamespace = p.Params["targetNamespace"]
		serviceAccount = p.Params["serviceAccount"]
	}
	if p.TemplateID == "argocd-tenant-dynamic-namespaces" {
		serviceAccountNamespace = p.Params["namespace"]
		serviceAccount = p.Params["serviceAccount"]
		accessNamespace = p.Params["namespacePattern"]
		if strings.ContainsAny(accessNamespace, "*?[]") {
			return p
		}
	}
	if p.Params["targetNamespace"] != "" && p.Params["targetServiceAccount"] != "" && strings.Contains(p.TemplateID, "argocd-control-plane") {
		serviceAccountNamespace = p.Params["targetNamespace"]
		accessNamespace = p.Params["targetNamespace"]
		serviceAccount = p.Params["targetServiceAccount"]
	}
	if serviceAccountNamespace == "" || accessNamespace == "" || serviceAccount == "" {
		return p
	}
	_, client, ok := s.clientForClusterNoHTTP(p.ClusterID)
	if !ok {
		return p
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	checks := defaultAccessChecks(accessNamespace)
	results, err := client.ValidateServiceAccount(checkCtx, serviceAccountNamespace, serviceAccount, checks)
	if err != nil {
		p.Validation = []ValidationCheck{{
			Allowed:        false,
			Namespace:      accessNamespace,
			Verb:           "validate",
			Resource:       "subjectaccessreviews",
			Reason:         err.Error(),
			ServiceAccount: serviceAccountNamespace + "/" + serviceAccount,
		}}
		p = s.store.PutPlan(p)
		return p
	}
	p.Validation = fromKubeChecks(results)
	p = s.store.PutPlan(p)
	return p
}

func cleanupBindingRefs(findings []Finding) []ResourceSnapshot {
	refs := map[string]ResourceSnapshot{}
	for _, finding := range findings {
		if !cleanupEligibleRule(finding.RuleID) {
			continue
		}
		ref, ok := parseBindingResource(finding.Resource)
		if !ok {
			continue
		}
		key := ref.Kind + "/" + ref.Namespace + "/" + ref.Name
		refs[key] = ref
	}
	out := make([]ResourceSnapshot, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref)
	}
	return out
}

func cleanupEligibleRule(ruleID string) bool {
	switch ruleID {
	case "cluster-admin", "wildcard-rbac", "cluster-write", "pod-exec", "privilege-escalation", "argocd-controller-cluster-admin":
		return true
	default:
		return false
	}
}

func parseBindingResource(resource string) (ResourceSnapshot, bool) {
	parts := strings.Split(resource, "/")
	switch len(parts) {
	case 2:
		if parts[0] == "ClusterRoleBinding" && parts[1] != "" {
			return ResourceSnapshot{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding", Name: parts[1], Exists: true}, true
		}
	case 3:
		if parts[1] == "RoleBinding" && parts[0] != "" && parts[2] != "" {
			return ResourceSnapshot{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding", Namespace: parts[0], Name: parts[2], Exists: true}, true
		}
	}
	return ResourceSnapshot{}, false
}

func defaultAccessChecks(namespace string) []kube.AccessCheck {
	resources := []struct {
		group    string
		resource string
		verbs    []string
	}{
		{group: "", resource: "configmaps", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{group: "", resource: "services", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{group: "", resource: "secrets", verbs: []string{"get", "create", "update", "patch", "delete"}},
		{group: "apps", resource: "deployments", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{group: "apps", resource: "statefulsets", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{group: "batch", resource: "jobs", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{group: "networking.k8s.io", resource: "ingresses", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
	}
	checks := []kube.AccessCheck{}
	for _, res := range resources {
		for _, verb := range res.verbs {
			checks = append(checks, kube.AccessCheck{Namespace: namespace, Verb: verb, Group: res.group, Resource: res.resource})
		}
	}
	return checks
}

func fromKubeChecks(in []kube.AccessCheck) []ValidationCheck {
	out := make([]ValidationCheck, 0, len(in))
	for _, check := range in {
		out = append(out, ValidationCheck{
			Allowed:        check.Allowed,
			Namespace:      check.Namespace,
			Verb:           check.Verb,
			Group:          check.Group,
			Resource:       check.Resource,
			Name:           check.Name,
			Reason:         check.Reason,
			ServiceAccount: check.ServiceAccount,
		})
	}
	return out
}

func fromKubeSnapshots(in []kube.ObjectSnapshot) []ResourceSnapshot {
	out := make([]ResourceSnapshot, 0, len(in))
	for _, snapshot := range in {
		out = append(out, ResourceSnapshot{
			APIVersion: snapshot.APIVersion,
			Kind:       snapshot.Kind,
			Namespace:  snapshot.Namespace,
			Name:       snapshot.Name,
			YAML:       snapshot.YAML,
			Exists:     snapshot.Exists,
		})
	}
	return out
}

func toKubeSnapshots(in []ResourceSnapshot) []kube.ObjectSnapshot {
	out := make([]kube.ObjectSnapshot, 0, len(in))
	for _, snapshot := range in {
		out = append(out, kube.ObjectSnapshot{
			APIVersion: snapshot.APIVersion,
			Kind:       snapshot.Kind,
			Namespace:  snapshot.Namespace,
			Name:       snapshot.Name,
			YAML:       snapshot.YAML,
			Exists:     snapshot.Exists,
		})
	}
	return out
}

func (s *Server) clientForClusterNoHTTP(id string) (Cluster, *kube.Client, bool) {
	c, ok := s.store.GetCluster(id)
	if !ok {
		return Cluster{}, nil, false
	}
	var client *kube.Client
	var err error
	if c.Kubeconfig == "IN_CLUSTER" {
		client, _, err = kube.NewInCluster()
	} else {
		client, _, err = kube.NewFromKubeconfig(c.Kubeconfig)
	}
	if err != nil {
		return Cluster{}, nil, false
	}
	return c, client, true
}
