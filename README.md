# RBAC Governance Console

A Kubernetes RBAC governance console for discovering over-privileged platform tools, previewing safer permission templates, and applying selected permissions through Fairwinds RBAC Manager.

## Current Capabilities

- Import a cluster with kubeconfig.
- Run in-cluster when deployed into Kubernetes.
- Auto-register the current cluster when running in-cluster.
- Basic user and tenant model with default `admin` platform administrator.
- Chinese and English UI language switching.
- Detect whether Fairwinds RBAC Manager is installed.
- Discover common tool workloads:
  - Argo CD
  - Jenkins
  - Prometheus
  - Loki
  - Promtail, Grafana Agent, and Alloy-style log collectors
- Analyze ServiceAccount RBAC risks:
  - `cluster-admin`
  - wildcard permissions
  - broad cluster write permissions
  - Secret read access
  - `pods/exec`
  - `bind`, `escalate`, and `impersonate`
- Preview built-in permission templates.
- Register custom tool discovery profiles.
- Register custom role templates from YAML.
- Create remediation plans.
- Run SubjectAccessReview compatibility validation and show allowed/denied checks.
- Apply generated `ClusterRole` and `RBACDefinition` resources.
- Save rollback snapshots before apply and roll back applied plans.
- Detect Argo CD sync impersonation status, AppProject destination ServiceAccounts, and application-controller `cluster-admin`.
- Record audit events.
- Persist local state in `data/state.json`.

## Run Locally

Install frontend dependencies once:

```bash
cd web
npm install
cd ..
```

Build the Vue frontend and run the Gin backend:

```bash
make build
./bin/rbac-manager
```

Open:

```text
http://localhost:8080
```

If the environment blocks the default Go build cache, run:

```bash
GOCACHE="$PWD/.gocache" go run -buildvcs=false ./cmd/server
```

For frontend-only development:

```bash
npm --prefix web run dev
```

The Vite dev server proxies `/api` to the Gin backend on `localhost:8080`.

## In-Cluster Mode

When the app runs inside Kubernetes, it automatically registers the current cluster as `in-cluster`. Users do not need to upload kubeconfig for that cluster. The UI also provides a "Use current in-cluster" / "接入当前集群" action for explicit registration.

## Extending Tools And Roles

New tools are registered as Tool Profiles. A profile contains:

- `type`: stable tool type, for example `trivy`
- `name`: display name
- `matchText`: comma-separated tokens matched against workload name, namespace, and common labels
- `recommendedTemplateIds`: templates recommended for this tool

New roles are registered as custom templates. A template contains metadata, required params such as `namespace` and `serviceAccount`, and YAML resources rendered at plan time.

## State

By default, local state is written to:

```text
data/state.json
```

Override it with:

```bash
DATA_FILE=/secure/path/state.json go run ./cmd/server
```

The MVP stores kubeconfig content in this local state file. The file is written with `0600` permissions, but production deployments should replace this with Kubernetes Secrets, Vault, or another secret manager.

## Template Approach

Built-in templates live in code and are rendered only when a user previews or creates a plan. Applying a plan writes generated resources to the target cluster.

Fairwinds RBAC Manager is used for binding management through `RBACDefinition`. The console creates or applies the referenced `ClusterRole` resources itself.

## Argo CD Approach

Argo CD is handled differently from read-only tools.

Recommended production model:

- Argo CD control-plane ServiceAccount gets read/watch permissions and narrowly scoped impersonation.
- Tenant deployment ServiceAccounts get namespace-scoped permissions.
- Fairwinds RBAC Manager manages ServiceAccount bindings.
- Argo CD AppProjects use `destinationServiceAccounts`.
- Kubernetes RBAC remains the final enforcement layer.

The current implementation detects Argo CD version hints, sync impersonation enablement, AppProject `destinationServiceAccounts`, and application-controller `cluster-admin`. It also includes templates for control-plane impersonation and tenant namespace deployers.

## Project Structure

```text
cmd/server/          Gin server entrypoint
internal/app/        API handlers, store, templates, planner, RBAC analyzer
internal/kube/       Kubernetes client, discovery, apply logic
web/                 Vue 3 + Vite frontend
docs/                Product plan and development task breakdown
```

## Production Hardening Still Needed

- PostgreSQL or another durable database.
- Secret manager integration for kubeconfig storage.
- OIDC or enterprise SSO authentication.
- More granular tenant administration UI.
- AppProject patch apply workflow for Argo CD sync impersonation.
- Approval flow for high-risk templates.
- Helm chart.
