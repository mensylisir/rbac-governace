# Kind Test Run

Context:

```text
kind-rbac-manager-test
```

Verified on:

```text
Kubernetes v1.35.0
```

## Fairwinds RBAC Manager

Installed from the official deploy manifests:

```bash
kubectl apply -f https://raw.githubusercontent.com/FairwindsOps/rbac-manager/master/deploy/0_namespace.yaml --context kind-rbac-manager-test
kubectl apply -f https://raw.githubusercontent.com/FairwindsOps/rbac-manager/master/deploy/1_rbac.yaml --context kind-rbac-manager-test
kubectl apply -f https://raw.githubusercontent.com/FairwindsOps/rbac-manager/master/deploy/2_crd.yaml --context kind-rbac-manager-test
kubectl apply -f https://raw.githubusercontent.com/FairwindsOps/rbac-manager/master/deploy/3_deployment.yaml --context kind-rbac-manager-test
```

The official deployment uses `quay.io/reactiveops/rbac-manager:v1`. The kind node initially had an invalid container-local proxy of `127.0.0.1:10808`; updating containerd/kubelet proxy to `http://192.168.31.34:10808` allowed the image pull to complete.

## Tool Fixtures

Applied:

```bash
kubectl apply -f test/kind-tool-fixtures.yaml --context kind-rbac-manager-test
```

Fixtures include:

- Argo CD-like application controller with `cluster-admin`.
- Jenkins-like workload with wildcard, Secret read, and `pods/exec`.
- Prometheus-like read-only workload.
- Loki-like workload without high-risk RBAC.

## Full Argo CD Install

Installed the official Argo CD manifest into the existing `argocd` namespace:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml --context kind-rbac-manager-test
```

The install created the real Argo CD components:

- `argocd-application-controller` StatefulSet
- `argocd-server`
- `argocd-repo-server`
- `argocd-redis`
- `argocd-dex-server`
- `argocd-applicationset-controller`
- `argocd-notifications-controller`

The `applicationsets.argoproj.io` CRD failed on this cluster because the manifest annotations exceeded the Kubernetes annotation size limit. The core Argo CD components still rolled out successfully, and the scanner detected the real Argo CD ServiceAccounts and RBAC.

## Verified Behaviors

- Cluster import through the Gin API.
- In-cluster auto-registration after deploying the app into the kind cluster.
- RBAC Manager CRD detection.
- Tool discovery for Argo CD, Jenkins, Prometheus, and Loki.
- Full Argo CD multi-component discovery after installing the official manifest.
- Risk findings:
  - Argo CD: application-controller wildcard RBAC, sync impersonation disabled, no AppProject destination ServiceAccounts.
  - Jenkins: wildcard RBAC, Secret read, `pods/exec`.
  - Prometheus: low risk.
  - Loki: low risk.
- Plan creation for `jenkins-namespace-deployer`.
- Apply of generated `ClusterRole` and `RBACDefinition`.
- RBAC Manager reconciliation into a namespace `RoleBinding`.
- Rollback of the plan and removal of RBAC Manager-generated binding.
- Custom Tool Profile registration through `/api/tool-profiles`.
- Custom role template registration through `/api/templates`.
- Chinese/English UI build with visible language switch.

## App Deployment In Kind

Built a local image and loaded it into kind:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o bin/rbac-manager ./cmd/server
npm --prefix web run build
docker build -f Dockerfile.local -t rbac-governance:test .
kind load docker-image rbac-governance:test --name rbac-manager-test
kubectl apply -f deploy/kubernetes.yaml --context kind-rbac-manager-test
kubectl set image deployment/rbac-governance -n rbac-governance app=rbac-governance:test --context kind-rbac-manager-test
```

Verified:

```bash
kubectl port-forward -n rbac-governance svc/rbac-governance 18084:80 --context kind-rbac-manager-test
curl -s http://localhost:18084/api/clusters
```

The deployed app returned an auto-detected `in-cluster` cluster with RBAC Manager status `installed`.
