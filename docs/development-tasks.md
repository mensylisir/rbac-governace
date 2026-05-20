# Development Task Breakdown

This task list is written as vertical slices. Each completed slice should be demoable on its own.

## 1. Bootstrap The Web Console Skeleton

Type: AFK

Blocked by: None

Build:

- Go API service skeleton.
- React + TypeScript UI skeleton.
- Local development command.
- Basic layout with navigation for Clusters, Tools, Templates, Plans, and Audit.
- Health endpoint and placeholder dashboard.

Acceptance criteria:

- The app starts locally.
- The UI can call the backend health endpoint.
- Navigation layout exists for all MVP pages.

## 2. Import And Test A Kubernetes Cluster

Type: AFK

Blocked by: Task 1

Build:

- Cluster import page.
- Backend endpoint for kubeconfig import.
- Connection test using Kubernetes discovery API.
- Persist cluster metadata and encrypted or secret-backed credential reference.
- Show cluster connection status in the cluster list.

Acceptance criteria:

- A user can import a kubeconfig.
- The backend can test API connectivity.
- The cluster appears in the UI with status.
- Failed imports show actionable errors.

## 3. Detect Fairwinds RBAC Manager Installation

Type: AFK

Blocked by: Task 2

Build:

- Detection of RBAC Manager CRD and controller deployment.
- Display installation status on the cluster list and cluster detail page.
- Add warning when the cluster cannot apply RBAC Manager resources.

Acceptance criteria:

- The app detects whether `RBACDefinition` CRD exists.
- The UI shows installed, missing, or unknown.
- Missing RBAC Manager blocks apply operations that require it.

## 4. Discover Tool Instances And ServiceAccounts

Type: AFK

Blocked by: Task 2

Build:

- Discovery engine for Argo CD, Jenkins, Prometheus, Loki, and common log collectors.
- Identify namespace, workload, labels, and ServiceAccount.
- Tool scan page with discovered tool cards.

Acceptance criteria:

- A cluster scan lists detected tools.
- Each detected tool shows namespace and ServiceAccount.
- Unknown workloads with notable permissions can be shown as custom tools.

## 5. Analyze RBAC Bindings And Produce Findings

Type: AFK

Blocked by: Task 4

Build:

- RBAC analyzer for ServiceAccount bindings.
- Risk rules for `cluster-admin`, wildcard permissions, secrets, write access, cluster-scoped resources, `pods/exec`, `bind`, `escalate`, and `impersonate`.
- Findings API and UI severity display.

Acceptance criteria:

- A ServiceAccount bound to `cluster-admin` is flagged high risk.
- Read-only Prometheus-style permissions are low risk.
- Findings explain why the permission is risky.

## 6. Add Built-In Template Registry

Type: AFK

Blocked by: Task 1

Build:

- Codebase-backed template directory.
- Template metadata parser and validator.
- Built-in templates for Argo CD, Jenkins, Prometheus, Loki.
- Template Center page.

Acceptance criteria:

- Built-in templates are loaded at startup.
- Templates are visible in the UI.
- Invalid templates fail validation with clear errors.

## 7. Render Template Preview For A Tool

Type: AFK

Blocked by: Tasks 4, 6

Build:

- Template parameter form.
- Renderer that produces Kubernetes YAML and Fairwinds `RBACDefinition`.
- YAML preview and structured summary.
- No cluster writes yet.

Acceptance criteria:

- A user can select a discovered tool and a matching template.
- The UI shows rendered YAML.
- Required parameters are validated before rendering.

## 8. Generate A Remediation Plan With Diff

Type: AFK

Blocked by: Task 7

Build:

- Remediation plan entity.
- Server-side comparison between current cluster state and rendered resources.
- Plan detail page with create, update, delete, and unchanged sections.
- Explicit warning for high-risk changes.

Acceptance criteria:

- The app creates a plan without applying it.
- The plan shows exactly what would change.
- High-risk templates or removals are visually marked.

## 9. Apply A Namespace-Scoped RBAC Manager Template

Type: AFK

Blocked by: Tasks 3, 8

Build:

- Apply engine for safe server-side apply of generated Role, ServiceAccount, and `RBACDefinition` resources.
- Audit event creation.
- Apply result page.

Acceptance criteria:

- A namespace-scoped template can be applied to a test cluster.
- The resulting `RBACDefinition` exists in the cluster.
- The audit log records the user, cluster, tool, template, and result.

## 10. Add Rollback Snapshot Support

Type: AFK

Blocked by: Task 9

Build:

- Capture current resources before apply.
- Store rollback snapshot.
- Rollback endpoint and UI action.
- Audit rollback attempts and results.

Acceptance criteria:

- Applying a plan stores a rollback snapshot.
- The user can roll back a previously applied plan.
- Rollback events appear in the audit log.

## 11. Add Argo CD Version And Impersonation Detection

Type: AFK

Blocked by: Task 4

Build:

- Detect Argo CD version.
- Detect `argocd-cm` setting `application.sync.impersonation.enabled`.
- Detect AppProject `destinationServiceAccounts`.
- Detect application-controller `cluster-admin` binding.
- Display Argo CD-specific governance status.

Acceptance criteria:

- The UI shows whether Argo CD sync impersonation is available and enabled.
- The UI warns when Argo CD has `cluster-admin`.
- The UI does not recommend sync impersonation if the installed version does not support it.

## 12. Generate Argo CD Tenant Impersonation Plan

Type: HITL

Blocked by: Tasks 8, 11

Build:

- Plan generation for tenant ServiceAccount.
- Fairwinds RBAC Manager binding for tenant namespace permissions.
- AppProject patch for `destinationServiceAccounts`.
- Optional argocd-cm patch to enable impersonation.
- Explicit non-removal of old `cluster-admin` binding by default.

Acceptance criteria:

- A platform admin can generate a tenant deployment plan for Argo CD.
- The plan includes ServiceAccount, RBACDefinition, and AppProject changes.
- Removing old `cluster-admin` is a separate explicit step.

## 13. Validate Planned Permissions With SubjectAccessReview

Type: AFK

Blocked by: Task 12

Build:

- Validation engine that checks whether the selected ServiceAccount can perform required operations.
- Initial validation against selected resource kinds and namespaces.
- UI panel showing pass and fail checks.

Acceptance criteria:

- The app can report whether a tenant deployer can create/update common workload resources.
- Missing permissions are shown before apply.
- Failed validation blocks unsafe Argo CD migration by default.

## 14. Add Console Tenants And User Roles

Type: AFK

Blocked by: Task 1

Build:

- Tenant entity.
- User role model for Platform Admin, Tenant Admin, Viewer, and Auditor.
- Backend authorization checks.
- UI filtering by allowed clusters and namespaces.

Acceptance criteria:

- Tenant Admins only see assigned scopes.
- Viewers cannot apply plans.
- Auditors can view audit records but cannot change resources.

## 15. Add Custom Template Creation

Type: AFK

Blocked by: Task 6

Build:

- Create custom template page.
- Duplicate built-in template as custom.
- YAML validation.
- Tenant ownership.
- Draft and published states.

Acceptance criteria:

- A user can create a custom template.
- A user can duplicate a built-in template.
- Only published templates are available for application.

## 16. Build The Audit Log Page

Type: AFK

Blocked by: Task 9

Build:

- Audit event list.
- Filters by cluster, tool, user, action, and status.
- Event detail view with resource summary.

Acceptance criteria:

- Apply and rollback events are visible.
- The audit log is filterable.
- Audit records include enough detail for security review.

## 17. Polish The MVP User Experience

Type: HITL

Blocked by: Tasks 1-16

Build:

- Empty states.
- Loading and error states.
- Risk severity colors.
- Safer wording for dangerous actions.
- Final visual pass on Clusters, Tools, Templates, Plans, and Audit pages.

Acceptance criteria:

- A first-time user can import a cluster, scan tools, inspect findings, preview a template, apply it, and view the audit record.
- Dangerous actions require clear confirmation.
- The UI is dense, readable, and operationally useful.

