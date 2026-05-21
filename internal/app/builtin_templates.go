package app

func builtins() []Template {
	commonParams := []TemplateParam{
		{Name: "namespace", Label: "Namespace", Required: true},
		{Name: "serviceAccount", Label: "ServiceAccount", Required: true},
	}
	return []Template{
		{
			ID: "namespace-editor", Tool: "common", Name: "Namespace 编辑权限",
			Description: "Grants full edit access to common resources in a specific namespace.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: namespaceEditClusterRole("namespace-editor")}, {Kind: "RBACDefinition", Template: rbacDefinition("namespace-editor", "namespace-editor")}},
		},
		{
			ID: "argocd-control-plane", Tool: "argocd", Name: "Argo CD 控制面权限",
			Description: "Replaces the default wildcard permissions for argocd-application-controller with read-only get/list/watch across all resources.",
			Scope:       "cluster", RiskLevel: "medium", Builtin: true,
			Params: []TemplateParam{
				{Name: "namespace", Label: "Argo CD Namespace", Required: true},
				{Name: "controllerServiceAccount", Label: "Argo CD Controller ServiceAccount", Required: true},
			},
			Resources: []TemplateResource{
				{Kind: "ClusterRole", Template: argocdControllerReadClusterRole},
				{Kind: "RBACDefinition", Template: argocdControllerReadRBACDefinition},
			},
		},
		{
			ID: "argocd-static-tenant", Tool: "argocd", Name: "Argo CD 静态租户权限",
			Description: "Creates one Argo CD tenant sync ServiceAccount, AppProject, namespace-scoped tenant RBAC, and the exact controller impersonation permission for that tenant.",
			Scope:       "mixed", RiskLevel: "high", Builtin: true,
			Params: []TemplateParam{
				{Name: "namespace", Label: "Argo CD Namespace", Required: true},
				{Name: "controllerServiceAccount", Label: "Argo CD Controller ServiceAccount", Required: true},
				{Name: "serviceAccount", Label: "Tenant Sync ServiceAccount", Required: true},
				{Name: "targetNamespace", Label: "Tenant Namespace", Required: true},
				{Name: "sourceRepo", Label: "Allowed source repository", Required: true, Default: "*"},
			},
			Resources: []TemplateResource{
				{Kind: "ServiceAccount", Template: argocdCentralTenantSyncServiceAccount},
				{Kind: "AppProject", Template: argocdTenantAppProject},
				{Kind: "ClusterRole", Template: namespaceEditClusterRole("argocd-static-tenant")},
				{Kind: "RBACDefinition", Template: argocdTenantSyncRBACDefinition},
				{Kind: "Role", Template: argocdTenantImpersonateRole},
				{Kind: "RBACDefinition", Template: argocdControllerImpersonateRBACDefinition},
			},
		},
		{
			ID: "argocd-dynamic-tenant", Tool: "argocd", Name: "Argo CD 动态租户权限",
			Description: "Creates a tenant AppProject and grants the tenant ServiceAccount access to namespaces selected by label through RBAC Manager.",
			Scope:       "mixed", RiskLevel: "high", Builtin: true,
			Params: []TemplateParam{
				{Name: "namespace", Label: "Argo CD Namespace", Required: true},
				{Name: "controllerServiceAccount", Label: "Argo CD Controller ServiceAccount", Required: true},
				{Name: "serviceAccount", Label: "Tenant Sync ServiceAccount", Required: true},
				{Name: "tenant", Label: "Tenant name", Required: true},
				{Name: "sourceRepo", Label: "Allowed source repository", Required: true, Default: "*"},
			},
			Resources: []TemplateResource{
				{Kind: "ServiceAccount", Template: argocdCentralTenantSyncServiceAccount},
				{Kind: "AppProject", Template: argocdDynamicTenantAppProject},
				{Kind: "ClusterRole", Template: namespaceEditClusterRole("argocd-dynamic-tenant")},
				{Kind: "RBACDefinition", Template: argocdDynamicTenantSyncRBACDefinition},
				{Kind: "Role", Template: argocdTenantImpersonateRole},
				{Kind: "RBACDefinition", Template: argocdDynamicControllerImpersonateRBACDefinition},
			},
		},
		{
			ID: "jenkins-agent-manager", Tool: "jenkins", Name: "Jenkins agent manager",
			Description: "Lets Jenkins manage build agent Pods in one namespace.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: jenkinsAgentClusterRole}, {Kind: "RBACDefinition", Template: rbacDefinition("jenkins-agent", "jenkins-agent-manager")}},
		},
		{
			ID: "jenkins-namespace-edit", Tool: "jenkins", Name: "Jenkins namespace edit",
			Description: "Lets Jenkins deploy common workload resources in one namespace.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: namespaceEditClusterRole("jenkins-namespace-edit")}, {Kind: "RBACDefinition", Template: rbacDefinition("jenkins-deploy", "jenkins-namespace-edit")}},
		},
		{
			ID: "prometheus-cluster-reader", Tool: "prometheus", Name: "Prometheus cluster reader",
			Description: "Read-only cluster discovery for Prometheus without write permissions.",
			Scope:       "cluster", RiskLevel: "low", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: prometheusClusterReader}, {Kind: "RBACDefinition", Template: clusterRBACDefinition("prometheus-reader", "prometheus-cluster-reader")}},
		},
		{
			ID: "prometheus-namespace-reader", Tool: "prometheus", Name: "Prometheus namespace reader",
			Description: "Namespace-scoped discovery for Prometheus.",
			Scope:       "namespace", RiskLevel: "low", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: prometheusNamespaceReader}, {Kind: "RBACDefinition", Template: rbacDefinition("prometheus-ns-reader", "prometheus-namespace-reader")}},
		},
		{
			ID: "loki-namespace-reader", Tool: "loki", Name: "Loki namespace reader",
			Description: "Minimal namespace metadata read permissions for Loki components.",
			Scope:       "namespace", RiskLevel: "low", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: metadataNamespaceReader("loki-namespace-reader")}, {Kind: "RBACDefinition", Template: rbacDefinition("loki-reader", "loki-namespace-reader")}},
		},
		{
			ID: "promtail-cluster-metadata-reader", Tool: "log-collector", Name: "Log collector metadata reader",
			Description: "Read-only metadata discovery for Promtail, Grafana Agent, or Alloy.",
			Scope:       "cluster", RiskLevel: "low", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: promtailClusterReader}, {Kind: "RBACDefinition", Template: clusterRBACDefinition("log-collector-reader", "promtail-cluster-metadata-reader")}},
		},
	}
}

func namespaceEditClusterRole(name string) string {
	return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ` + name + `
rules:
  - apiGroups: [""]
    resources: ["configmaps", "services", "serviceaccounts", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses", "networkpolicies"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]`
}

const argocdTenantSyncServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccount }}
  namespace: {{ .targetNamespace }}`

const argocdCentralTenantSyncServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccount }}
  namespace: {{ .namespace }}`

const argocdTenantAppProject = `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: {{ dns .targetNamespace }}-tenant
  namespace: {{ .namespace }}
spec:
  sourceRepos:
    - '{{ .sourceRepo }}'
  destinations:
    - server: https://kubernetes.default.svc
      namespace: {{ .targetNamespace }}
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: {{ .targetNamespace }}
      defaultServiceAccount: {{ .namespace }}:{{ .serviceAccount }}
  namespaceResourceWhitelist:
    - group: ''
      kind: ConfigMap
    - group: ''
      kind: Secret
    - group: ''
      kind: Service
    - group: ''
      kind: ServiceAccount
    - group: apps
      kind: Deployment
    - group: apps
      kind: StatefulSet
    - group: apps
      kind: DaemonSet
    - group: batch
      kind: Job
    - group: batch
      kind: CronJob
    - group: networking.k8s.io
      kind: Ingress
    - group: networking.k8s.io
      kind: NetworkPolicy`

const argocdDynamicTenantAppProject = `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: {{ dns .tenant }}-tenant
  namespace: {{ .namespace }}
spec:
  sourceRepos:
    - '{{ .sourceRepo }}'
  destinations:
    - server: https://kubernetes.default.svc
      namespace: '*'
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: '*'
      defaultServiceAccount: {{ .namespace }}:{{ .serviceAccount }}
  namespaceResourceWhitelist:
    - group: ''
      kind: ConfigMap
    - group: ''
      kind: Secret
    - group: ''
      kind: Service
    - group: ''
      kind: ServiceAccount
    - group: apps
      kind: Deployment
    - group: apps
      kind: StatefulSet
    - group: apps
      kind: DaemonSet
    - group: batch
      kind: Job
    - group: batch
      kind: CronJob
    - group: networking.k8s.io
      kind: Ingress
    - group: networking.k8s.io
      kind: NetworkPolicy`

const argocdTenantSyncRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .targetNamespace }}-argocd-static-tenant
rbacBindings:
  - name: {{ dns .targetNamespace }}-{{ dns .serviceAccount }}-argocd-static-tenant
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .targetNamespace }}
        clusterRole: argocd-static-tenant`

const argocdDynamicTenantSyncRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .tenant }}-argocd-dynamic-tenant
rbacBindings:
  - name: {{ dns .tenant }}-{{ dns .serviceAccount }}-argocd-dynamic-tenant
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespaceSelector:
          matchLabels:
            tenant: "{{ .serviceAccount }}"
        clusterRole: argocd-dynamic-tenant`

const argocdControllerReadClusterRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: argocd-controller-read
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
  - nonResourceURLs: ["*"]
    verbs: ["get"]`

const argocdControllerReadRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: argocd-controller-read
rbacBindings:
  - name: {{ dns .namespace }}-{{ dns .controllerServiceAccount }}-argocd-controller-read
    subjects:
      - kind: ServiceAccount
        name: {{ .controllerServiceAccount }}
        namespace: {{ .namespace }}
    clusterRoleBindings:
      - clusterRole: argocd-controller-read`

const argocdTenantImpersonateRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-impersonate-{{ dns .serviceAccount }}
  namespace: {{ .namespace }}
rules:
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["impersonate"]
    resourceNames: ["{{ .serviceAccount }}"]`

const argocdControllerImpersonateRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .targetNamespace }}-argocd-controller-impersonate
rbacBindings:
  - name: {{ dns .targetNamespace }}-{{ dns .controllerServiceAccount }}-impersonate
    subjects:
      - kind: ServiceAccount
        name: {{ .controllerServiceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .namespace }}
        role: argocd-impersonate-{{ dns .serviceAccount }}`

const argocdDynamicControllerImpersonateRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .tenant }}-argocd-controller-impersonate
rbacBindings:
  - name: {{ dns .tenant }}-{{ dns .controllerServiceAccount }}-impersonate
    subjects:
      - kind: ServiceAccount
        name: {{ .controllerServiceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .namespace }}
        role: argocd-impersonate-{{ dns .serviceAccount }}`

func rbacDefinition(prefix, clusterRole string) string {
	return `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .namespace }}-` + prefix + `
rbacBindings:
  - name: {{ dns .namespace }}-{{ dns .serviceAccount }}-` + prefix + `
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .namespace }}
        clusterRole: ` + clusterRole
}

func clusterRBACDefinition(prefix, clusterRole string) string {
	return `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .namespace }}-` + prefix + `
rbacBindings:
  - name: {{ dns .namespace }}-{{ dns .serviceAccount }}-` + prefix + `
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    clusterRoleBindings:
      - clusterRole: ` + clusterRole
}


const jenkinsAgentClusterRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jenkins-agent-manager
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["secrets", "configmaps"]
    verbs: ["get", "list", "watch"]`

const prometheusClusterReader = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus-cluster-reader
rules:
  - apiGroups: [""]
    resources: ["nodes", "nodes/metrics", "services", "endpoints", "pods", "namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - nonResourceURLs: ["/metrics", "/metrics/cadvisor"]
    verbs: ["get"]`

const prometheusNamespaceReader = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus-namespace-reader
rules:
  - apiGroups: [""]
    resources: ["services", "endpoints", "pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]`

func metadataNamespaceReader(name string) string {
	return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ` + name + `
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces"]
    verbs: ["get", "list", "watch"]`
}

const promtailClusterReader = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: promtail-cluster-metadata-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces", "nodes"]
    verbs: ["get", "list", "watch"]`
