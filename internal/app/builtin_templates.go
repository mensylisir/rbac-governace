package app

func builtins() []Template {
	commonParams := []TemplateParam{
		{Name: "namespace", Label: "Namespace", Required: true},
		{Name: "serviceAccount", Label: "ServiceAccount", Required: true},
	}
	return []Template{
		{
			ID: "argocd-tenant-namespace-deployer", Tool: "argocd", Name: "Argo CD tenant namespace deployer",
			Description: "Legacy single-namespace template. Prefer the tenant project template for new Argo CD tenants.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: namespaceDeployerClusterRole("argocd-tenant-namespace-deployer")}, {Kind: "RBACDefinition", Template: rbacDefinition("argocd-tenant-sync", "argocd-tenant-namespace-deployer")}},
		},
		{
			ID: "argocd-control-plane-read-only", Tool: "argocd", Name: "Argo CD control-plane read-only baseline",
			Description: "Base Argo CD controller template for a new installation. Grants read/watch only through RBAC Manager. Tenant ServiceAccounts and AppProjects are configured separately.",
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
			ID: "argocd-tenant-sync-project", Tool: "argocd", Name: "Argo CD tenant sync project",
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
				{Kind: "ClusterRole", Template: namespaceDeployerClusterRole("argocd-tenant-sync-project")},
				{Kind: "RBACDefinition", Template: argocdTenantSyncRBACDefinition},
				{Kind: "Role", Template: argocdTenantImpersonateRole},
				{Kind: "RBACDefinition", Template: argocdControllerImpersonateRBACDefinition},
			},
		},
		{
			ID: "argocd-tenant-dynamic-namespaces", Tool: "argocd", Name: "Argo CD tenant dynamic namespaces",
			Description: "Creates a tenant AppProject for a namespace pattern and grants the tenant ServiceAccount access to namespaces selected by label through RBAC Manager.",
			Scope:       "mixed", RiskLevel: "high", Builtin: true,
			Params: []TemplateParam{
				{Name: "namespace", Label: "Argo CD Namespace", Required: true},
				{Name: "controllerServiceAccount", Label: "Argo CD Controller ServiceAccount", Required: true},
				{Name: "serviceAccount", Label: "Tenant Sync ServiceAccount", Required: true},
				{Name: "tenant", Label: "Tenant name", Required: true},
				{Name: "namespacePattern", Label: "Allowed namespace pattern", Required: true},
				{Name: "tenantLabelKey", Label: "Namespace label key", Required: true, Default: "tenant"},
				{Name: "tenantLabelValue", Label: "Namespace label value", Required: true},
				{Name: "sourceRepo", Label: "Allowed source repository", Required: true, Default: "*"},
			},
			Resources: []TemplateResource{
				{Kind: "ServiceAccount", Template: argocdCentralTenantSyncServiceAccount},
				{Kind: "AppProject", Template: argocdDynamicTenantAppProject},
				{Kind: "ClusterRole", Template: namespaceDeployerClusterRole("argocd-tenant-sync-project")},
				{Kind: "RBACDefinition", Template: argocdDynamicTenantSyncRBACDefinition},
				{Kind: "Role", Template: argocdTenantImpersonateRole},
				{Kind: "RBACDefinition", Template: argocdDynamicControllerImpersonateRBACDefinition},
			},
		},
		{
			ID: "argocd-control-plane-read-impersonate", Tool: "argocd", Name: "Argo CD control-plane read and impersonate",
			Description: "Grants Argo CD read/watch permissions and namespace-scoped impersonation of a specific tenant ServiceAccount. AppProject changes are still required.",
			Scope:       "mixed", RiskLevel: "high", Builtin: true,
			Params: []TemplateParam{
				{Name: "namespace", Label: "Argo CD Namespace", Required: true},
				{Name: "serviceAccount", Label: "Argo CD Controller ServiceAccount", Required: true},
				{Name: "targetNamespace", Label: "Tenant SA Namespace", Required: true},
				{Name: "targetServiceAccount", Label: "Tenant SA", Required: true},
			},
			Resources: []TemplateResource{
				{Kind: "ClusterRole", Template: argocdControllerReadClusterRole},
				{Kind: "RBACDefinition", Template: argocdControlPlaneReadRBACDefinition},
				{Kind: "Role", Template: argocdControlPlaneImpersonateRole},
				{Kind: "RBACDefinition", Template: argocdControlPlaneImpersonateRBACDefinition},
			},
		},
		{
			ID: "jenkins-agent-manager", Tool: "jenkins", Name: "Jenkins agent manager",
			Description: "Lets Jenkins manage build agent Pods in one namespace.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: jenkinsAgentClusterRole}, {Kind: "RBACDefinition", Template: rbacDefinition("jenkins-agent", "jenkins-agent-manager")}},
		},
		{
			ID: "jenkins-namespace-deployer", Tool: "jenkins", Name: "Jenkins namespace deployer",
			Description: "Lets Jenkins deploy common workload resources in one namespace.",
			Scope:       "namespace", RiskLevel: "medium", Builtin: true, Params: commonParams,
			Resources: []TemplateResource{{Kind: "ClusterRole", Template: namespaceDeployerClusterRole("jenkins-namespace-deployer")}, {Kind: "RBACDefinition", Template: rbacDefinition("jenkins-deploy", "jenkins-namespace-deployer")}},
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

func namespaceDeployerClusterRole(name string) string {
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
      namespace: '{{ .namespacePattern }}'
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: '{{ .namespacePattern }}'
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
  name: {{ dns .targetNamespace }}-argocd-tenant-sync-project
rbacBindings:
  - name: {{ dns .targetNamespace }}-{{ dns .serviceAccount }}-argocd-tenant-sync-project
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .targetNamespace }}
        clusterRole: argocd-tenant-sync-project`

const argocdDynamicTenantSyncRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: {{ dns .tenant }}-argocd-tenant-sync-project
rbacBindings:
  - name: {{ dns .tenant }}-{{ dns .serviceAccount }}-argocd-tenant-sync-project
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespaceSelector:
          matchLabels:
            {{ .tenantLabelKey }}: {{ .tenantLabelValue }}
        clusterRole: argocd-tenant-sync-project`

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

const argocdControlPlaneReadRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: argocd-control-plane-read-{{ dns .namespace }}-{{ dns .serviceAccount }}
rbacBindings:
  - name: {{ dns .namespace }}-{{ dns .serviceAccount }}-argocd-control-plane-read
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    clusterRoleBindings:
      - clusterRole: argocd-controller-read`

const argocdControlPlaneImpersonateRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-impersonate-{{ dns .targetServiceAccount }}
  namespace: {{ .targetNamespace }}
rules:
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["impersonate"]
    resourceNames: ["{{ .targetServiceAccount }}"]`

const argocdControlPlaneImpersonateRBACDefinition = `apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: argocd-control-plane-impersonate-{{ dns .targetNamespace }}-{{ dns .targetServiceAccount }}
rbacBindings:
  - name: {{ dns .namespace }}-{{ dns .serviceAccount }}-impersonate-{{ dns .targetNamespace }}-{{ dns .targetServiceAccount }}
    subjects:
      - kind: ServiceAccount
        name: {{ .serviceAccount }}
        namespace: {{ .namespace }}
    roleBindings:
      - namespace: {{ .targetNamespace }}
        role: argocd-impersonate-{{ dns .targetServiceAccount }}`

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
