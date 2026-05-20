# RBAC Governance Console Product Plan

## Problem Statement

Platform teams need to reduce excessive Kubernetes permissions granted to tools such as Argo CD, Jenkins, Prometheus, Loki, and future internal tools. Many tools are installed with broad permissions, and Argo CD is especially risky because it commonly runs with `cluster-admin` while also exposing its own user, project, and sync model.

The product should provide a simple web console that discovers installed tools, analyzes their effective Kubernetes RBAC, recommends safer templates, and applies the selected permissions through Fairwinds RBAC Manager. It must support multiple clusters and multi-tenant access, where different users can see and manage different scopes.

## Product Goals

- Detect excessive permissions on common platform tools.
- Provide built-in permission templates for known tools.
- Allow tenant-specific and custom templates for new tools.
- Support multi-cluster management, not only in-cluster operation.
- Support multi-tenant platform access control.
- Apply changes only after preview, validation, and explicit confirmation.
- Keep the UI simple enough for fast implementation and safe daily use.

## Non-Goals For MVP

- Full approval workflow engine.
- Full GitOps pull request mode.
- Cloud-specific onboarding for EKS, GKE, or AKS.
- Full policy language replacement for OPA/Kyverno.
- Automatic removal of dangerous permissions without human confirmation.

## Core Concepts

### Cluster

A Kubernetes cluster imported into the console. The MVP should support:

- In-cluster connection.
- Kubeconfig import.
- Connection test.
- Detection of Fairwinds RBAC Manager installation.
- Optional installation guidance for RBAC Manager.

### Tool

A known or custom workload that uses Kubernetes permissions.

Initial built-in tool profiles:

- Argo CD
- Jenkins
- Prometheus
- Loki
- Promtail or Grafana Agent style log collectors

Future tools should be added without changing the core scan and apply pipeline.

### Permission Template

A reusable definition that describes recommended RBAC resources and Fairwinds RBAC Manager bindings.

Templates should be stored in the codebase as versioned built-ins, then loaded into the application at startup or migration time. User-created templates should be stored in the database.

Built-in templates are not directly editable. Users can duplicate them into custom templates.

### Finding

A scan result that identifies a permission risk, such as:

- `cluster-admin` binding.
- Wildcard verb, resource, or API group.
- Write access to cluster-scoped resources.
- Secret read access.
- `pods/exec` access.
- `bind`, `escalate`, or `impersonate` access.
- Unexpected cross-namespace write permission.

### Remediation Plan

A generated change plan that includes:

- Resources to create.
- Resources to update.
- Bindings to add.
- Bindings to remove or disable.
- Fairwinds `RBACDefinition` resources to apply.
- Required Argo CD AppProject changes where relevant.
- Compatibility checks and expected impact.

## Argo CD Strategy

Argo CD needs special handling. It should not be treated as a normal fixed-permission tool.

### Recommended Model

Use Argo CD sync impersonation for multi-tenant deployments:

- The Argo CD control-plane ServiceAccount keeps only the permissions needed to run Argo CD, read/watch target resources, and impersonate explicitly allowed tenant ServiceAccounts.
- Each tenant gets one or more deployment ServiceAccounts.
- Fairwinds RBAC Manager binds tenant ServiceAccounts to limited namespace permissions.
- Argo CD AppProjects use `destinationServiceAccounts` to select the ServiceAccount used for sync.
- AppProjects also restrict destinations, source repositories, namespace resources, and cluster resources.

### Version And Feature Checks

The console must check:

- Argo CD version.
- Whether sync impersonation is supported.
- Whether sync impersonation is enabled.
- Whether `argocd-cm` contains `application.sync.impersonation.enabled: "true"`.
- Whether AppProjects already use `destinationServiceAccounts`.
- Whether the application-controller ServiceAccount still has `cluster-admin`.

Sync impersonation should only be recommended when the installed Argo CD version supports it. If unsupported, the console should recommend safer legacy patterns, such as tighter AppProject restrictions and scoped cluster credentials, but not claim full sync impersonation support.

### Argo CD Permission Profiles

Initial Argo CD templates:

- `argocd-control-plane-read-impersonate`: read/watch plus impersonate selected tenant ServiceAccounts.
- `argocd-tenant-namespace-deployer`: namespace-scoped app deployment permissions.
- `argocd-platform-admin-deployer`: controlled cluster-level platform permissions.
- `argocd-breakglass-cluster-admin`: high-risk emergency admin profile, disabled by default.

### Argo CD Safety Flow

The console must not immediately remove `cluster-admin`.

Recommended flow:

1. Detect current Argo CD ServiceAccounts and bindings.
2. Detect Applications, AppProjects, destinations, and managed resource kinds.
3. Generate tenant ServiceAccounts and RBAC Manager definitions.
4. Generate AppProject patches.
5. Run SubjectAccessReview or dry-run checks where possible.
6. Apply new permissions.
7. Observe sync health.
8. Remove old `cluster-admin` binding only after explicit confirmation.

### Enforcement Layers

Argo CD should enforce:

- Which repositories are allowed.
- Which destinations are allowed.
- Which resource kinds are allowed.
- Which sync ServiceAccount is selected.

Kubernetes RBAC remains the final enforcement layer:

- Whether Argo CD can impersonate the chosen ServiceAccount.
- Whether the impersonated ServiceAccount can create, update, patch, delete, or watch the requested resource.

## Jenkins Strategy

Jenkins is simpler than Argo CD but still risky because pipelines may execute arbitrary deployment logic.

Recommended split:

- Jenkins controller ServiceAccount: manages agent Pods only in the CI namespace.
- Jenkins deploy ServiceAccount: namespace-scoped deployment permissions.
- Jenkins platform admin ServiceAccount: optional restricted cluster-level permissions for platform pipelines.

Initial Jenkins templates:

- `jenkins-agent-manager`: create/delete/watch agent Pods in one namespace.
- `jenkins-namespace-deployer`: deploy workloads to one namespace.
- `jenkins-platform-admin`: high-risk controlled platform deployment profile.

Risk checks:

- `cluster-admin`.
- Ability to create privileged Pods.
- Broad Secret read access.
- Cross-namespace write access.
- Kubeconfig credentials mounted into pipelines.

## Prometheus Strategy

Prometheus should usually be read-only.

Initial templates:

- `prometheus-cluster-reader`: get/list/watch metrics-related resources.
- `prometheus-namespace-reader`: namespace-scoped discovery.

Risk checks:

- Any write permission.
- Secret read permission unless explicitly required.
- `cluster-admin`.
- Wildcard verbs or resources.

## Loki And Log Collector Strategy

Loki itself often needs little Kubernetes RBAC. Promtail or Grafana Agent style collectors usually need metadata read permissions.

Initial templates:

- `loki-namespace-reader`.
- `promtail-metadata-reader`.
- `promtail-cluster-metadata-reader`.

Risk checks:

- Unexpected write permission.
- Secret read permission.
- `cluster-admin`.
- HostPath and privileged Pod usage should be surfaced as workload security findings, even though they are not pure RBAC issues.

## Template Model

Built-in templates should live in the codebase, for example:

```text
templates/
  argocd/
  jenkins/
  prometheus/
  loki/
```

A template should include:

- Stable ID.
- Display name.
- Tool type.
- Scope: cluster, namespace, or mixed.
- Risk level.
- Description.
- Required parameters.
- Kubernetes resources to create or patch.
- Fairwinds `RBACDefinition` resources.
- Compatibility checks.
- Warnings.

Custom templates should support:

- Tenant ownership.
- Versioning.
- Draft and published state.
- YAML validation.
- Preview rendering.

## Multi-Tenant Console Model

The console needs its own authorization layer separate from Kubernetes RBAC.

Initial roles:

- Platform Admin: manages all clusters, templates, and remediation plans.
- Tenant Admin: manages assigned namespaces and tool permissions.
- Viewer: reads findings and plans.
- Auditor: reads findings, plans, and audit logs only.

Tenant scope should include:

- Allowed clusters.
- Allowed namespaces.
- Allowed tools.
- Allowed template types.

## MVP Pages

### Cluster List

Shows:

- Cluster name.
- Connection status.
- RBAC Manager installation status.
- High, medium, and low risk finding counts.
- Last scan time.

### Cluster Import

Supports:

- In-cluster setup.
- Kubeconfig import.
- Connection test.
- Permission check.
- RBAC Manager installation detection.

### Tool Scan

Shows cards for discovered tools:

- Tool name.
- Namespace.
- ServiceAccount.
- Current highest-risk binding.
- Finding count.
- Recommended action.

### Permission Governance

Shows:

- Current effective permissions.
- Risk findings.
- Recommended templates.
- YAML preview.
- Diff view.
- Compatibility checks.
- Apply button.
- Rollback snapshot.

### Template Center

Shows:

- Built-in templates.
- Custom templates.
- Tool filters.
- Risk level.
- Scope.
- Duplicate built-in template as custom.

### Audit Log

Shows:

- Who scanned.
- Who generated a plan.
- Who applied a plan.
- What changed.
- Previous snapshot.
- Result status.

## Backend Architecture

Suggested stack:

- Go backend.
- `client-go` for Kubernetes access.
- PostgreSQL for clusters, templates, findings, plans, users, tenants, and audit logs.
- Kubernetes Secret or external secret backend for kubeconfig storage.
- Background scanner workers.
- REST API for MVP.

Core modules:

- Cluster connection manager.
- Tool discovery engine.
- RBAC analyzer.
- Template registry.
- Template renderer.
- Remediation planner.
- Kubernetes apply engine.
- Argo CD integration.
- Audit logger.
- Console authorization.

## Frontend Architecture

Suggested stack:

- React.
- Vite.
- TypeScript.
- Ant Design or shadcn/ui.
- Monaco Editor for YAML preview.

Design direction:

- Security governance dashboard.
- Dense but clean operational UI.
- Risk severity should be visually clear.
- Avoid decorative marketing layout.
- Main screen should be the usable product, not a landing page.

## Data Model Draft

Primary entities:

- `clusters`
- `cluster_credentials`
- `tenants`
- `users`
- `user_tenant_roles`
- `tool_instances`
- `service_accounts`
- `rbac_bindings`
- `findings`
- `templates`
- `template_versions`
- `remediation_plans`
- `remediation_plan_items`
- `audit_events`

## API Draft

Initial endpoints:

- `GET /api/clusters`
- `POST /api/clusters/import`
- `POST /api/clusters/{id}/test`
- `POST /api/clusters/{id}/scan`
- `GET /api/clusters/{id}/tools`
- `GET /api/tools/{id}/findings`
- `GET /api/templates`
- `POST /api/templates`
- `POST /api/templates/{id}/render`
- `POST /api/tools/{id}/plans`
- `GET /api/plans/{id}`
- `POST /api/plans/{id}/validate`
- `POST /api/plans/{id}/apply`
- `POST /api/plans/{id}/rollback`
- `GET /api/audit-events`

## Safety Requirements

- Scans must be read-only.
- Applying a plan must require explicit confirmation.
- High-risk templates must require an additional warning confirmation.
- The system must store a rollback snapshot before applying changes.
- The system must not silently remove `cluster-admin`.
- Argo CD changes must include AppProject and sync impersonation checks.
- All apply operations must be audited.

## Implementation Phases

### Phase 1: MVP

- Multi-cluster import.
- Tool discovery for Argo CD, Jenkins, Prometheus, Loki.
- RBAC risk scanning.
- Built-in template registry.
- Render and preview templates.
- Apply namespace-scoped templates through Fairwinds RBAC Manager.
- Basic UI pages.
- Audit log.

### Phase 2: Argo CD Advanced Governance

- Argo CD sync impersonation enablement flow.
- AppProject patch generation.
- Application resource-kind analysis.
- SubjectAccessReview validation.
- Safe removal workflow for old `cluster-admin`.

### Phase 3: Enterprise Controls

- Approval workflow.
- Time-limited breakglass permissions.
- GitOps PR mode.
- Drift detection.
- Policy-as-code integration.
- Cloud provider cluster import.

