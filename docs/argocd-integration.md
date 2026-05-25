# ArgoCD 集成配置指南

本文档描述完整的 ArgoCD 与 Keycloak 对接流程，以及启用租户级 RBAC Impersonation 所需的配置。

---

## 一、Keycloak 配置

### 1. 创建 Client

在 Keycloak 中创建一个名为 `argocd` 的 OpenID Connect Client。

### 2. 创建 groups Scope

1. 登录 Keycloak 管理控制台
2. 左侧菜单选择 **Client Scopes**
3. 点击 **Create client scope**：
   - **Name**: `groups`
   - **Protocol**: openid-connect
   - **Include in token scope**: On
4. 点击 **Save**
5. 在 groups Scope 详情页的 **Mappers** 标签页中：
   - 点击 **Configure a new mapper**，选择 **Group Membership**
   - **Name**: `groups`
   - **Token Claim Name**: `groups`
   - **Full group path**: Off
   - **Add to ID token**: On
   - **Add to access token**: On
   - **Add to userinfo**: On
6. 点击 **Save**

### 3. 将 groups Scope 分配给 ArgoCD Client

1. 左侧菜单选择 **Clients**
2. 点击 **argocd** 客户端
3. 进入 **Client scopes** 标签页
4. 点击 **Add client scope**，勾选 **groups**，选择 **Default** 添加

### 4. 创建用户和 Group

1. 创建用户（如 `liminggang`）
2. 创建 Group（如 `team-a-admins`）
3. 将用户加入 Group

---

## 二、ArgoCD 配置

### 1. 配置 OIDC Secret

```bash
kubectl -n argocd patch secret argocd-secret \
  --patch='{"stringData": { "oidc.keycloak.clientSecret": "<client-secret>" }}'
```

### 2. 配置 argocd-cm

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  oidc.config: |
    name: Keycloak
    issuer: https://<keycloak-host>/auth/realms/<realm>
    clientID: argocd
    clientSecret: $oidc.keycloak.clientSecret
    requestedScopes: ["openid", "profile", "email", "groups"]
    rbacGroupClaim: "groups"
    userNameKey: "preferred_username"
    userIDKey: "preferred_username"
```

关键字段说明：

- `requestedScopes`: 必须包含 `groups`，否则 Keycloak 不会返回 group 信息
- `rbacGroupClaim`: 指定从哪个 claim 读取 group 信息，此处为 `groups`
- `userNameKey` / `userIDKey`: 使用 `preferred_username` 作为用户名展示

> ⚠️ **注意**：确保 Keycloak 的 `groups` Client Scope 中**没有** `username-to-groups` 等 Mapper，否则会污染 `groups` claim，导致 ArgoCD 无法正确识别 group。

### 3. 启用 Impersonation（必需）

ArgoCD Application Controller 使用 **租户 ServiceAccount** 身份来同步应用，此功能默认关闭，必须显式启用：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # ArgoCD v2.12+ 标准开关
  application.sync.impersonation.enabled: "true"

  # 部分发行版需要同时配置
  applicationcontroller.enable.impersonation: "true"
```

修改后重启 Controller：

```bash
kubectl rollout restart statefulset/argocd-application-controller -n argocd
```

#### 为什么必须启用

禁用此开关时，Controller 始终使用自身的 ServiceAccount（`argocd-application-controller`）执行同步。当 Controller 缺少 namespace 写入权限时，会导致权限被拒绝错误，破坏租户隔离。

启用后，Controller 会：
1. 从 Application 中识别目标集群和 namespace
2. 查找 AppProject 中的 `destinationServiceAccounts` 配置
3. 为指定的租户 ServiceAccount 创建 **合成 token**
4. 使用该 token 应用资源，确保操作在租户的 RBAC 范围内执行

### 4. 配置 ArgoCD RBAC（可选但推荐）

在 `argocd-rbac-cm` 中添加全局策略，确保未分组用户默认无任何权限：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:readonly, applications, get, */*, allow
    p, role:readonly, projects, get, *, allow
    g, team-a-admins, role:admin
```

---

## 三、租户模板工作原理

### argocd-static-tenant / argocd-dynamic-tenant 模板生成以下资源：

| 资源 | 说明 |
|------|------|
| ServiceAccount | 租户同步用的 SA，位于 ArgoCD namespace |
| AppProject | 租户项目，配置 `destinationServiceAccounts` 指向租户 SA |
| ClusterRole | namespace 级别的工作负载权限（Deployment、Service、ConfigMap 等） |
| RBACDefinition | 将 ClusterRole 绑定给租户 SA |
| Role | 允许 ArgoCD Controller impersonate 租户 SA |
| RBACDefinition | 将 impersonate Role 绑定给 `argocd-application-controller` |

完整的用户请求链路：

1. 用户通过 Keycloak 登录 ArgoCD
2. Keycloak 返回 ID Token，其中 `groups` claim 包含用户所属 group
3. ArgoCD 根据 `rbacGroupClaim: "groups"` 读取用户的 group
4. ArgoCD 匹配 AppProject `roles[].groups`，确定用户可见的项目
5. 用户在项目中创建 Application，目标 namespace 受 `AppProject.destinations` 限制
6. ArgoCD Controller 启用 impersonation 后，使用租户 SA 执行同步
7. K8s API Server 根据租户 SA 的 RBAC 权限决定是否允许资源创建

---

## 验证清单

- [ ] Keycloak 中 `groups` Client Scope 创建正确（Group Membership Mapper，Token Claim Name 为 `groups`）
- [ ] `username-to-*` Mappers 已删除，不会污染 `groups` claim
- [ ] ArgoCD Client 的 Client Scopes 中包含 `groups`（Default）
- [ ] `argocd-cm` 中 `rbacGroupClaim` 设置为 `"groups"`
- [ ] `argocd-cm` 中 `application.sync.impersonation.enabled` 为 `"true"`
- [ ] Controller 已重启
- [ ] 租户模板中的 `AppProject` 包含 `destinationServiceAccounts`
- [ ] 租户 SA 拥有目标 namespace 的 `namespace-edit` ClusterRole 权限
- [ ] Controller 拥有 impersonate 租户 SA 的权限
