# RBAC Governance 问题清单

扫描日期：2026-05-21

---

## 🔴 严重 — 安全 / 稳定性

- [ ] **ISSUE-001: 认证体系完全缺失，任何人可伪造 `X-User: admin` 获取超级管理员权限**
  - 位置：`internal/app/auth.go:15-24`
  - `currentUser()` 仅从 Header `X-User` 读取身份，无签名、无验证、无会话管理，默认回退为 `admin`。
  - **修复建议**：接入企业 OIDC / LDAP，或至少基于共享密钥的 JWT / mTLS 校验。

- [ ] **ISSUE-002: Kubeconfig 明文存储在 JSON 文件中，备份或逃逸即可泄露所有集群凭证**
  - 位置：`internal/app/store.go`
  - `data/state.json` 中以明文存储 kubeconfig 内容，权限 `0o600` 不足以应对容器逃逸或备份场景。
  - **修复建议**：使用 Kubernetes Secret 托管，或基于主密钥加密的文件存储（如 AES-GCM + env 密钥）。

- [ ] **ISSUE-003: `readJSON` 中 `defer r.Body.Close()` 导致 Body 二次关闭**
  - 位置：`internal/app/server.go:811-814`
  - Gin 会在 handler 返回后自动关闭 `Request.Body`，额外 `defer` 可能触发 panic（取决于 Go 版本）。
  - **修复建议**：移除 `defer r.Body.Close()`，信任 Gin 的生命周期管理。

---

## 🟠 架构缺陷

- [ ] **ISSUE-004: 单体内存存储（JSON 文件）性能差、无事务、无限 Audit Log 增长、崩溃可能损坏文件**
  - 位置：`internal/app/store.go:28-377`
  - 每次变更全量序列化写入，锁内执行 IO，高并发阻塞；audit 无限追加无截断。
  - **修复建议**：引入 SQLite / BadgerDB / etcd，启用 WAL 或定时快照；audit 按时间窗口自动归档/截断。

- [ ] **ISSUE-005: HTTP Server 没有优雅关闭逻辑，SIGTERM 时强制断开连接**
  - 位置：`cmd/server/main.go:12-26`
  - `http.Server` 未绑定 `Shutdown`，未捕获 `SIGTERM`/`SIGINT`。
  - **修复建议**：引入 `signal.Notify` + `httpServer.Shutdown(context.WithTimeout(...))`。

- [ ] **ISSUE-006: API 路径无版本前缀，未来演进困难**
  - 位置：`internal/app/server.go:37-78`
  - 所有路由为 `/api/...`，缺少 `/api/v1/...` 版本控制。
  - **修复建议**：将现有路由统一加 `/v1` 前缀（或做兼容双写）。

- [ ] **ISSUE-007: `TemplateRegistry` 非线程安全，自定义模板创建与并发读取存在 map 冲突风险**
  - 位置：`internal/app/template_registry.go:11-38`
  - `templates` map 无锁保护；`handleCreateTemplate` 写入与 `handleRenderTemplate` 读取可能并发冲突。
  - **修复建议**：加 `sync.RWMutex`，或换为 `sync.Map`。

- [ ] **ISSUE-008: Plan Apply 不是原子操作，YAML 应用成功但 Cleanup 失败会导致状态不一致**
  - 位置：`internal/app/server.go:630-653`
  - ApplyYAML 成功后状态为 `applied`，若后续 Cleanup 失败则标记 `failed`；但新权限已生效，且 rollback snapshot 不包含 Cleanup 删除的资源。
  - **修复建议**：将 Cleanup 也纳入 snapshot（记录被删资源的完整 YAML），或改为事务性两阶段应用。

---

## 🟡 逻辑与性能问题

- [ ] **ISSUE-009: `clientForCluster` 每次 API 调用都重建 K8s Client，无连接缓存**
  - 位置：`internal/app/server.go:696-714`
  - `kube.NewFromKubeconfig` / `kube.NewInCluster` 每次重新解析 kubeconfig、新建 RESTClient，在 20-45s 超时接口中性能极差。
  - **修复建议**：维护 `map[string]*kube.Client` 缓存，带 TTL 或主动失效机制。

- [ ] **ISSUE-010: RBAC 发现遍历全部 RoleBindings 和 ClusterRoleBindings，大集群性能灾难**
  - 位置：`internal/kube/client.go:312-348`
  - `RulesForServiceAccount` 用 `List` 拉取全量绑定，无 `FieldSelector` 过滤。
  - **修复建议**：改用 `FieldSelector`（如 `subject.name=xxx`）或引入 Informer/Indexer 缓存。

- [ ] **ISSUE-011: `hasServiceAccountSubject` 未处理 Group / User 绑定，间接绑定 ServiceAccount 的 Group 不会被发现**
  - 位置：`internal/kube/client.go:350-357`
  - 只匹配 `Kind: "ServiceAccount"`，忽略 `Kind: "Group"` 中包含 service account group 的情况。
  - **修复建议**：若需要完整分析，应同时检查 `system:serviceaccounts:*` 等系统 group 的绑定。

- [ ] **ISSUE-012: `resolveTemplateParams` 依赖内存中的 tools 列表，存在竞态**
  - 位置：`internal/app/server.go:525-546`
  - `findArgoController` 读取 `s.store.ListTools()`，可能被 `ReplaceClusterTools` 并发修改而返回不一致结果。
  - **修复建议**：在 `resolveTemplateParams` 内复制 snapshot，或在 store 层保证一致性读取。

- [ ] **ISSUE-013: Discovery / REST Mapper 无缓存，每次 Apply/Snapshot 重复查询**
  - 位置：`internal/kube/client.go:543-554`
  - `gvrFor` 每次调用 Discovery API，对多资源 YAML 反复查询。
  - **修复建议**：使用 `restmapper.NewDiscoveryRESTMapper` 或 `k8s.io/client-go/restmapper` 缓存 GVK->GVR 映射。

- [ ] **ISSUE-014: `argocd-dynamic-tenant` 的 `namespacePattern` 为通配符时跳过了权限校验，但其他模板无类似处理**
  - 位置：`internal/app/plan_validation.go:21-27`
  - 通配符 `namespacePattern` 导致 SAR 校验被跳过，合理；但 `defaultAccessChecks` 对通配符无意义，且其他模板未统一处理。
  - **修复建议**：在 `validatePlanBestEffort` 中明确区分“可校验”与“跳过校验”的模板类型，并在 UI / API 中给出说明。

---

## 🟢 代码质量 / 可维护性

- [ ] **ISSUE-015: `ArgoCDPolicy` 与 `DefaultPolicy` 存在大量重复代码**
  - 位置：`internal/app/policy.go`
  - `Analyze` 方法几乎完全一致，只有 `exempts` 一处差异。
  - **修复建议**：用组合基类或提取公共函数（如 `defaultAnalyze(tool, rules, exempts)`）。

- [ ] **ISSUE-016: `newClient` 使用 `OrDie` 构造器，API 不可用时直接 panic**
  - 位置：`internal/kube/client.go:139-148`
  - `kubernetes.NewForConfigOrDie` 和 `dynamic.NewForConfigOrDie` 在集群 API 临时不可用时 panic 整个进程。
  - **修复建议**：改用 `kubernetes.NewForConfig` / `dynamic.NewForConfig`，显式处理错误。

- [ ] **ISSUE-017: 前端 `App.vue` 为 God File（1500+ 行），状态、API、UI 全部耦合**
  - 位置：`web/src/App.vue`
  - 所有页面逻辑、API 调用、类型定义、国际化、模板渲染集中在一个文件。
  - **修复建议**：按视图拆分组件（ClustersPage, ToolsPage, PlansPage...），提取 composable / store。

- [ ] **ISSUE-018: `PodSecurityFindings` 死代码**
  - 位置：`internal/kube/client.go:556-569`
  - 函数定义了但未被任何调用方引用。
  - **修复建议**：若后续不需要则删除；若要实现 PodSecurity 扫描则补充调用链路。

---

## 🔴 严重 — 安全（补充扫描）

- [ ] **ISSUE-019: 缺少 CSRF 保护，跨站请求可伪造敏感操作**
  - 位置：`internal/app/server.go:37-78`
  - 所有写操作（`POST /api/plans/*/apply`、`POST /api/clusters/*/scan` 等）均依赖 `X-User` Header，但无 CSRF Token、无 SameSite Cookie、无 Origin 校验。攻击者可通过钓鱼页面诱导管理员触发危险操作。
  - **修复建议**：引入 CSRF Token（双重提交 Cookie 模式），或要求所有写操作携带不可预测的 Token。

- [ ] **ISSUE-020: 请求体大小无限制，可导致 OOM / DoS**
  - 位置：`internal/app/server.go:811-814`
  - `readJSON` 直接解码 `r.Body`，没有 `http.MaxBytesReader` 限制。上传超大 kubeconfig 或恶意 JSON 可能导致内存耗尽。
  - **修复建议**：`r.Body = http.MaxBytesReader(w, r.Body, maxSize)`，kubeconfig 接口限制 1-2MB，一般 JSON 限制 256KB。

- [ ] **ISSUE-021: 缺少安全响应头，存在 XSS 点击劫持、MIME 混淆风险**
  - 位置：`internal/app/server.go:37-78`
  - 没有 `X-Frame-Options`、`X-Content-Type-Options`、`Content-Security-Policy`、`Strict-Transport-Security`。前端虽无 `v-html`，但 iframe 嵌入可导致点击劫持。
  - **修复建议**：Gin 中间件统一注入安全响应头。

- [ ] **ISSUE-022: 没有 API 速率限制，暴力破解 / 资源耗尽风险**
  - 位置：`cmd/server/main.go:12-26`
  - 全局无任何速率限制（ratelimit），扫描集群、创建计划、生成凭证等重操作可被无限制调用。
  - **修复建议**：引入基于 IP / 用户的 Token Bucket 限流器（如 `golang.org/x/time/rate`），对敏感接口设置更严格的配额。

---

## 🟠 架构缺陷（补充扫描）

- [ ] **ISSUE-023: `store.load()` JSON 反序列化无 schema 校验，字段增删可能导致数据丢失或静默失败**
  - 位置：`internal/app/store.go:323-357`
  - `json.Unmarshal` 直接映射到 `storeSnapshot`，缺失字段静默忽略，类型不匹配不报错。后续增加字段时旧数据加载后字段为空，可能导致空指针或业务异常。
  - **修复建议**：引入 schema version + migration 机制，或至少在关键字段缺失时返回有意义的错误。

- [ ] **ISSUE-024: Kubernetes 部署缺少 Pod 安全上下文、健康探针、资源限制**
  - 位置：`deploy/kubernetes.yaml:58-115`
  - Deployment 没有 `securityContext`（`runAsNonRoot`、`readOnlyRootFilesystem`、`allowPrivilegeEscalation: false`），没有 `livenessProbe`/`readinessProbe`，没有 `resources` 限制。
  - **修复建议**：添加 securityContext、探针、资源请求/限制；`data` 目录使用 `emptyDir` + 定期备份，或 StatefulSet + PVC。

- [ ] **ISSUE-025: 缺少 NetworkPolicy，Pod 网络完全开放**
  - 位置：`deploy/kubernetes.yaml`
  - 没有 `NetworkPolicy` 限制 `rbac-governance` Pod 的出站流量。被入侵后可直连集群内外任意服务。
  - **修复建议**：添加最小权限 NetworkPolicy，仅允许 8080 入站和必要的 K8s API 出站。

- [ ] **ISSUE-026: `systemNamespaces` 硬编码在代码中，不可配置**
  - 位置：`internal/app/server.go:727-732`
  - `kube-system`、`rbac-manager` 等系统命名空间白名单写死在 binary 中，不同环境无法调整。
  - **修复建议**：通过环境变量或配置文件注入系统命名空间列表。

- [ ] **ISSUE-027: 前端 `fetch` 没有超时、没有重试、没有取消机制**
  - 位置：`web/src/App.vue:499-504`
  - `api()` 函数直接 `fetch`，未传 `AbortController`，用户在慢网络下多次点击“Apply”会发起并发请求；`scanCluster` 等 45s+ 接口无前端超时提示。
  - **修复建议**：封装带 `AbortController` 和超时（如 60s）的 fetch 包装器，按钮 loading 状态绑定请求生命周期。

- [ ] **ISSUE-028: 前端 Vue 状态全部集中在单个 `reactive` 对象，无 Store / 状态规范化**
  - 位置：`web/src/App.vue:415-452`
  - 40+ 个字段挤在 `state` reactive 对象中，computed 之间隐式依赖，没有单向数据流。多个 watch、computed 链的副作用难以追踪。
  - **修复建议**：引入 Pinia 或拆分为独立的 composable（`useClusters()`、`usePlans()`、`useAuth()`），按模块分离状态。

- [ ] **ISSUE-029: 前端 `kubeconfig` 文本框内容在 `reactive` 中明文持有，无敏感输入脱敏**
  - 位置：`web/src/App.vue:454`
  - `importForm.kubeconfig` 在 Vue reactive 中全程明文，开发者工具 / Vue DevTools 可直接查看完整 kubeconfig。
  - **修复建议**：输入框使用 `type="password"` 样式或自定义敏感输入组件，提交后立即从内存中清除。

- [ ] **ISSUE-030: 前端缺少错误边界，Vue 组件渲染错误可导致整页白屏**
  - 位置：`web/src/App.vue`
  - 没有 Vue 3 `onErrorCaptured` 或错误边界组件，后端返回异常数据结构时 `computed` 抛出错误将导致整页崩溃。
  - **修复建议**：顶层包裹 ErrorBoundary 组件，拦截并降级渲染。

- [ ] **ISSUE-031: `tenantKubeconfig` 使用字符串拼接生成 YAML，存在格式和注入风险**
  - 位置：`internal/app/server.go:782-809`
  - 使用 `fmt.Sprintf` 拼接 kubeconfig YAML，如果 `clusterName`、`apiServer` 等参数包含特殊字符（换行、`:` 等），将产生无效或恶意 YAML。
  - **修复建议**：使用 `goccy/go-yaml` 或 `sigs.k8s.io/yaml` 结构化生成，避免字符串拼接。

- [ ] **ISSUE-032: `handleListPlans` 和 `handleListAudit` 没有权限过滤，任何用户可查看全部记录**
  - 位置：`internal/app/server.go:476-478`、`internal/app/server.go:692-694`
  - `/api/plans` 和 `/api/audit-events` 返回所有用户的计划/审计记录，tenant-admin / viewer 可能看到其他租户的敏感操作。
  - **修复建议**：按当前用户的 cluster / namespace 作用域过滤返回列表。

- [ ] **ISSUE-033: `validatePlanBestEffort` 的 `clientForClusterNoHTTP` 可能在锁内重建连接**
  - 位置：`internal/app/plan_validation.go:175-191`
  - 该辅助函数在 `validatePlanBestEffort` 中被调用，而 `validatePlanBestEffort` 可能从 `handleCreatePlan` 和 `handleValidatePlan` 调用；虽然不直接持有 store 锁，但连接重建仍会阻塞当前 goroutine 达数秒。
  - **修复建议**：与 ISSUE-009 一起，统一用 Client 缓存解决。

- [ ] **ISSUE-034: `newID` 使用 8 字节随机数，冲突概率虽低但无防重试**
  - 位置：`internal/app/store.go:65-71`
  - `rand.Read(b[:])` 生成 16 位 hex ID， Birthday Paradox 下在百万级数量时有微小冲突概率；冲突时直接覆盖原记录。
  - **修复建议**：冲突检测 + 重试，或使用时间戳 + 随机数 + 工作节点标识保证唯一性。

- [ ] **ISSUE-035: `plan.yaml` 字段存储完整渲染后的 YAML 字符串，重复存储浪费空间且无结构化**
  - 位置：`internal/app/models.go:128`
  - 每次创建 plan 都存储完整 YAML 字符串（可能数百 KB），无法做结构化查询或 diff；回滚时重新解析 YAML 字符串再做 Apply。
  - **修复建议**：存储结构化资源列表（`[]TemplateResource`），渲染和快照按需生成。

- [ ] **ISSUE-036: `applyPlan` 和 `rollbackPlan` 的后端操作成功但前端因网络断开未收到响应，状态不同步**
  - 位置：`web/src/App.vue:680-692`
  - Apply 成功后后端已变更集群状态，但若前端 fetch 超时或断开，`await refresh()` 未执行，UI 仍显示旧状态。用户可能重复点击 Apply。
  - **修复建议**：引入 Plan 状态轮询或 WebSocket 推送，不依赖单次请求响应完整性；Apply/Rollback 按钮在前端 loading 期间禁止操作。

- [ ] **ISSUE-037: `handleGetPlan` 没有权限校验，任何知道 Plan ID 的人可查看详情**
  - 位置：`internal/app/server.go:558-565`
  - `/api/plans/:id` 直接根据 ID 返回 plan，未检查当前用户是否有权查看该 plan 的 cluster/namespace。
  - **修复建议**：获取 plan 后校验 `authorizeNamespace(user, p.ClusterID, p.Params["namespace"])`。

- [ ] **ISSUE-038: `handleCreateTemplate` 允许自定义模板 ID 与内置模板冲突，覆盖内置模板**
  - 位置：`internal/app/template_registry.go:36-38`
  - `Add()` 直接用自定义模板覆盖 map 中的值，若用户创建 ID 为 `namespace-editor` 的自定义模板，将永久覆盖内置模板（重启后恢复但运行时受影响）。
  - **修复建议**：禁止自定义模板 ID 与内置模板冲突，或在 ID 前强制加 `custom-` 前缀。

- [ ] **ISSUE-039: `classify` 函数使用简单字符串包含匹配，会误匹配不相关工作负载**
  - 位置：`internal/kube/client.go:294-310`
  - `strings.Contains(text, "argocd")` 会匹配 `not-argocd-related` 等非预期名称；标签值为空字符串时参与拼接可能产生无意义匹配。
  - **修复建议**：使用前缀/后缀匹配、正则边界或白名单标签精确匹配。

- [ ] **ISSUE-040: `validatePlanBestEffort` 即使 SAR 全部失败也会创建 plan，未阻止高风险操作**
  - 位置：`internal/app/server.go:519-522`
  - 创建 plan 时调用 `validatePlanBestEffort(r.Context(), p)` 是“尽力”调用，即使 SAR 全部 denied，plan 状态仍为 `planned`，用户可直接 Apply。
  - **修复建议**：`handleCreatePlan` 中若 validation 结果全部 denied，plan 状态设为 `validation-failed` 并阻止 Apply；或在 Apply 前强制重新校验。

- [ ] **ISSUE-041: 审计日志是纯字符串消息，无结构化字段，无法做安全分析和告警**
  - 位置：`internal/app/server.go:716-718`
  - `audit(action, clusterID, toolID, planID, status, message)` 只有 6 个字符串字段，缺少用户 ID、请求 IP、耗时、差异对比等关键信息。
  - **修复建议**：扩展 `AuditEvent` 为结构化 JSON，包含 `userId`、`clientIP`、`requestDurationMs`、`beforeState`、`afterState` 等字段；支持导出到 SIEM / Loki。

- [ ] **ISSUE-042: `gin.Recovery()` 在生产环境可能暴露堆栈 trace，泄露代码路径**
  - 位置：`internal/app/server.go:40`
  - Gin Recovery 中间件在 panic 时会打印包含文件路径和代码行号的堆栈信息到响应或日志，攻击者可利用此信息定向攻击。
  - **修复建议**：自定义 Recovery Handler，响应中只返回 `"internal server error"`，将详细堆栈写入安全日志。

- [ ] **ISSUE-043: 前端 `@vitejs/plugin-vue` 被错误地放在 `dependencies` 中**
  - 位置：`web/package.json:12-14`
  - `@vitejs/plugin-vue` 是构建时依赖，不应出现在 `dependencies`（会被打包到产物中），应移到 `devDependencies`。
  - **修复建议**：将其移至 `devDependencies`。

- [ ] **ISSUE-044: 前端没有 Vue Router，所有视图通过 `state.view` 控制，刷新丢失状态、无法分享 URL**
  - 位置：`web/src/App.vue:415-420`
  - 状态保存在内存 reactive 中，页面刷新后回到默认 `clusters` 视图；用户无法通过 URL 直接分享某个 plan 详情或模板预览。
  - **修复建议**：引入 `vue-router`，视图状态与 URL path 同步（如 `/clusters`、`/plans/:id`）。

- [ ] **ISSUE-045: `busy` 是全局单一 ref，任何操作都会阻塞所有按钮**
  - 位置：`web/src/App.vue:472、837-840`
  - `busy` 是全局状态，一个耗时操作（如 `scanCluster`）触发后，整个 UI 所有按钮全部 disabled，即使是不相关的操作也无法点击。
  - **修复建议**：按操作粒度维护 loading 状态（`loadingActions: Set<string>`），每个按钮只绑定自己相关的 loading key。

- [ ] **ISSUE-046: 前端 `onTemplateChange` 只调用 `applyTemplateDefaults`，不清理旧模板残留参数，导致参数污染**
  - 位置：`web/src/App.vue:812-815`
  - 切换模板时不会清空上一个模板留下的参数值（如 `targetNamespace`），这些残留值可能注入到新模板的渲染中。
  - **修复建议**：`onTemplateChange` 中先 `for (const key of Object.keys(params)) params[key] = ''`，再注入新模板的默认值。

- [ ] **ISSUE-047: 服务器只监听 HTTP，kubeconfig 和 token 在网络中明文传输**
  - 位置：`cmd/server/main.go:13-20`
  - 没有 HTTPS/TLS 配置，kubeconfig、credential token 等敏感数据在传输中可被中间人截获。
  - **修复建议**：支持 TLS（通过环境变量 `TLS_CERT` / `TLS_KEY`），或至少强制在反向代理（如 nginx/istio）后终止 TLS 并在文档中说明。

- [ ] **ISSUE-048: 前端没有 WebSocket / SSE，所有状态更新依赖手动轮询或操作后刷新**
  - 位置：`web/src/App.vue:506-526`
  - `refresh()` 仅在挂载、手动点击或操作完成后调用；后台 plan 状态变更（如他人 Apply）不会自动推送到前端。
  - **修复建议**：引入 Server-Sent Events (`/api/events`) 或 WebSocket，推送 plan 状态变更、新集群发现等事件。

- [ ] **ISSUE-049: `frontendDist()` 在生产环境通过文件系统探测存在路径遍历风险**
  - 位置：`internal/app/server.go:101-116`
  - 如果 `FRONTEND_DIST` 环境变量被攻击者控制，可直接拼接任意路径并返回文件；`os.Executable()` 相对路径也可能被篡改。
  - **修复建议**：对 `FRONTEND_DIST` 做路径校验和 chroot 限制；使用 `filepath.Clean` + `strings.HasPrefix` 确保不越界。

- [ ] **ISSUE-050: 前端 `window.confirm` 是同步阻塞弹窗，现代浏览器可能阻止且用户体验差**
  - 位置：`web/src/App.vue:684、695`
  - Apply 和 Rollback 依赖 `window.confirm`，如果用户勾选“阻止此页面显示更多对话框”，确认逻辑将被跳过。
  - **修复建议**：使用自定义 Modal 组件替代 `window.confirm`，将确认状态纳入 Vue 响应式系统。

- [ ] **ISSUE-051: `go.mod` 声明 `go 1.26.3`，但 Go 最新稳定版为 1.24.x，该版本不存在，构建不可预测**
  - 位置：`go.mod:3`
  - Go 语言目前（2026-05）最新稳定版为 1.24.x，`go 1.26.3` 是虚构版本，会导致 `go mod` 解析、 toolchain 下载和 CI/CD 构建全部失败。
  - **修复建议**：修改为实际可用的 Go 版本（如 `go 1.24`），并运行 `go mod tidy` 修复 dependency graph。

- [ ] **ISSUE-052: Dockerfile 使用 `golang:1.26-alpine` 不存在的镜像，无法构建**
  - 位置：`Dockerfile:8`
  - 与 ISSUE-051 同源，`golang:1.26-alpine` 在 Docker Hub 上不存在，`docker build` 直接失败。
  - **修复建议**：改为 `golang:1.24-alpine`（或当时实际存在的版本）。

- [ ] **ISSUE-053: `bin/rbac-manager`（80MB+ 二进制文件）被 Git 追踪，导致仓库严重膨胀**
  - 位置：`bin/rbac-manager`
  - 该二进制文件在 `.gitignore` 中未被排除，且已被 `git ls-files` 确认追踪；每次重新编译后 `git diff` 会出现 80MB 变更。
  - **修复建议**：`git rm --cached bin/rbac-manager`，并在 `.gitignore` 新增 `bin/` 和 `*.exe`。

- [ ] **ISSUE-054: `go.mod` 中直接依赖（gin、go-yaml）被错误标记为 `indirect`**
  - 位置：`go.mod:21`、`go.mod:30`
  - `gin-gonic/gin` 在 `server.go:18` 直接 import，`goccy/go-yaml` 在 `client.go` 直接 import，但 `go.mod` 均标记为 `// indirect`。说明 `go.mod` 被手动编辑过且 `go mod tidy` 未正确执行。
  - **修复建议**：运行 `go mod tidy` 自动修复 direct/indirect 标记；若仍不正确，检查 repo 中是否有隐藏的 import。

- [ ] **ISSUE-055: 没有 CI/CD 配置（GitHub Actions / GitLab CI），无法做自动化测试和构建校验**
  - 位置：整个仓库
  - 没有 `.github/workflows`、`.gitlab-ci.yml` 等任何 CI 配置；`go test`、前端 build、Docker build 都依赖本地手动执行。
  - **修复建议**：添加 GitHub Actions workflow，执行 `go vet`、`go test`、`npm run build`、`docker build`、镜像扫描（Trivy）。

- [ ] **ISSUE-056: Makefile 缺少 `clean`、`lint`、`fmt` 目标，开发体验不完整**
  - 位置：`Makefile`
  - 没有 `make clean` 清理构建产物，没有 `make lint` 运行 `go vet` / `golangci-lint`，没有 `make fmt` 运行 `gofmt`。
  - **修复建议**：补充 `clean`、`fmt`、`vet`、`lint` 目标；`test` 可拆分为 `test-backend` 和 `test-frontend` 并支持并行运行。

- [ ] **ISSUE-057: 使用标准库 `log.Printf` 而非结构化日志，无法做日志聚合和告警**
  - 位置：`cmd/server/main.go:22`、`internal/app/store.go:44,364,370,374`
  - 所有日志都是无结构的纯文本，无法被 Loki / ELK / CloudWatch 有效解析和筛选。
  - **修复建议**：引入 `slog`（Go 1.21+）或 `uber-go/zap`，输出 JSON 结构化日志，包含 `level`、`component`、`request_id` 等字段。

- [ ] **ISSUE-058: `go.mod` 中使用了非标准 YAML 库路径 `go.yaml.in/yaml/v2` 和 `go.yaml.in/yaml/v3`**
  - 位置：`go.mod:50-51`
  - `go.yaml.in` 不是标准 Go module 域名，可能是 typo 或引用了不稳定的 fork；标准路径应为 `gopkg.in/yaml.v2` / `gopkg.in/yaml.v3`。
  - **修复建议**：检查实际使用来源，替换为标准 YAML 库；运行 `go mod tidy` 清理无效依赖。

- [ ] **ISSUE-059: 前端 `index.html` 没有 CSP（Content-Security-Policy）meta tag**
  - 位置：`web/index.html`
  - 没有 `<meta http-equiv="Content-Security-Policy" ...>`，如果后端被 XSS 攻击注入脚本，浏览器不会阻止执行。
  - **修复建议**：添加合适的 CSP header（也可由后端 Gin 中间件统一注入），限制 script-src、style-src、connect-src。

- [ ] **ISSUE-060: Dockerfile 没有 HEALTHCHECK，Kubernetes 无法判断容器健康状态**
  - 位置：`Dockerfile:1-10`
  - 没有 `HEALTHCHECK` 指令，Kubernetes Deployment 无法配置 `livenessProbe` / `readinessProbe`（除非在 k8s yaml 中单独定义）。
  - **修复建议**：在 Dockerfile 中添加 `HEALTHCHECK CMD curl -f http://localhost:8080/api/health || exit 1`。

- [ ] **ISSUE-061: `frontendDist()` 的 `repoRoot()` 使用 `runtime.Caller` 运行时反射获取路径，不可复现**
  - 位置：`internal/app/server.go:93-99`
  - `runtime.Caller(0)` 依赖编译时的文件路径，在交叉编译、Docker 多阶段构建或重命名目录后路径可能完全错误。
  - **修复建议**：使用 `embed` 将前端资源打包到 binary 中（Go 1.16+），彻底消除文件系统路径依赖。

- [ ] **ISSUE-062: `handleScanCluster` 中 `client.ArgoCDStatus` 被串行调用，扫描大集群时性能差**
  - 位置：`internal/app/server.go:319`
  - 对每个 argocd workload 都调用 `ArgoCDStatus`，其中包含多次 K8s API 调用（ConfigMap、Deployment、AppProject List、RulesForServiceAccount）。在大集群中串行执行极慢。
  - **修复建议**：为 argocd 工作负载批量预取 ArgoCD 状态，或将 `ArgoCDStatus` 结果缓存后传入 `analyzeRules`。

- [ ] **ISSUE-063: `TenantCredentialRequest.Expiration` 使用 `int64` 但 frontend 传 `number`，存在精度问题**
  - 位置：`internal/app/models.go:29`
  - 前端 JS number 是 IEEE-754 double，对于大于 2^53 的整数会精度丢失；虽然当前最大值为 86400 不会触发，但字段类型设计不规范。
  - **修复建议**：改为 `string` 类型并在后端解析，或限制前端使用字符串输入框。

- [ ] **ISSUE-064: `handleCreatePlan` 中没有对 `req.Params` 做输入校验，用户可注入 Go template 占位符**
  - 位置：`internal/app/server.go:506-508`
  - `req.Params` 的字符串值直接传入 `templates.Render()` 中的 Go template，如果参数包含 `{{ . }}` 等特殊 Go template 语法，可能导致模板渲染异常或信息泄露。
  - **修复建议**：在 `Render()` 前对参数值做白名单校验或转义，禁止包含 Go template 语法字符。

- [ ] **ISSUE-065: `Store` 的 `load()` 在反序列化后没有 `TenantIDs -> Tenants` 的关联回填**
  - 位置：`internal/app/store.go:124-137`
  - `GetUser` 会将 `TenantIDs` 转换为 `Tenants` 切片，但 `load()` 加载后直接放入 `s.users`，`Tenants` 字段为空；虽然 `GetUser` 会回填，但如果其他代码直接读取 `s.users` map 则看不到租户关联。
  - **修复建议**：`load()` 完成后统一执行一次 ` hydrateUsers()` 回填，或在所有访问路径上强制使用 `GetUser()`。

- [ ] **ISSUE-066: 前端组件的类型定义全部集中在 `App.vue`，子组件通过 `import type from '../App.vue'` 引入，形成循环依赖风险**
  - 位置：`web/src/components/ClusterPanel.vue:2`、`web/src/components/GovernancePanel.vue:2`
  - 所有类型定义（`Cluster`、`Tool`、`Template`、`Plan`、`Me` 等）都在 `App.vue` 中，子组件反向 import `App.vue` 获取类型。这在大型重构时可能导致循环依赖（如果 App.vue 某天需要 import 某个子组件的类型）。
  - **修复建议**：将类型定义提取到独立的 `types.ts` 文件中，所有组件统一从 `types.ts` import。

- [ ] **ISSUE-067: 前端将翻译对象 `t` 作为深嵌套 prop 传递给所有子组件，并通过 `props.t.xxx` 访问，耦合严重**
  - 位置：`web/src/components/ClusterPanel.vue:4-11`、`web/src/components/GovernancePanel.vue:4-15`
  - 每个子组件都接收 `t: Record<string, unknown>`，且 `GovernancePanel` 甚至把 `t.localizedTemplateName` 当作函数调用（`(props.t as Record<string, unknown>).localizedTemplateName`）。这导致类型不安全、重构困难、IDE 无法自动补全。
  - **修复建议**：提取 `useI18n()` composable，子组件通过 composable 直接访问翻译；或至少将 `t` 改为严格类型化的接口。

- [ ] **ISSUE-068: `cleanupCandidates`、`statusLabel`、`messageLabel` 等辅助函数在多个组件中重复定义，没有提取为共享工具函数**
  - 位置：`web/src/components/ClusterPanel.vue:22-47`、`web/src/components/GovernancePanel.vue:25-...`
  - 组件拆分后，翻译辅助函数、时间格式化、清理候选过滤等逻辑仍散落在各组件中；任何翻译文案或过滤规则变更需要改多个文件。
  - **修复建议**：提取 `composables/useFormatters.ts`、`composables/useCleanup.ts` 等共享模块，并在所有组件中复用。

- [ ] **ISSUE-069: 前端状态全部通过 App.vue 中转，子组件深度嵌套 props/emit，没有状态管理或 provide/inject**
  - 位置：`web/src/App.vue:1100-1200`
  - App.vue 作为唯一的“状态提升中心”，向下传递几十个子 prop，子组件 emit 事件全部回传 App.vue 处理。随着功能增加，prop drilling 会越来越深，维护成本指数增长。
  - **修复建议**：引入 Pinia 或组合式函数（`useClustersStore`、`usePlansStore`），子组件直接从 store 读写状态，减少中间层传递。

- [ ] **ISSUE-070: Go 测试只有 17 个测试函数，集成测试和端到端测试完全缺失**
  - 位置：整个 `internal/app/*_test.go`
  - 现有测试以单元测试为主，没有：`kube.Client` 的集成测试（mock K8s API）、`Server` 路由的端到端测试（使用 httptest + mock store）、前端组件测试（Vitest / Vue Test Utils）。
  - **修复建议**：补充 `envtest` 或 `fakeclientset` 做 K8s 集成测试；为前端添加 Vitest + `@vue/test-utils` 组件测试。

- [ ] **ISSUE-071: `ClusterPanel.vue` 中 `importForm` 的 `v-model` 等效写法繁琐且未使用 `defineModel`**
  - 位置：`web/src/components/ClusterPanel.vue:62-63`
  - 使用 `:value` + `@input` + `emit('update:importForm', { ... })` 手动实现双向绑定，代码冗长且容易出错。
  - **修复建议**：Vue 3.4+ 使用 `defineModel()` 宏简化，或至少封装为 `v-model` 兼容的自定义输入组件。

- [ ] **ISSUE-072: `store.seedDefaults()` 在 `NewStore()` 中无条件执行，会覆盖用户已删除的默认 tenant 或用户**
  - 位置：`internal/app/store.go:50-63`
  - 如果管理员删除了默认的 `platform` tenant 或 `admin` user，重启进程后 `seedDefaults()` 会重新创建它们，造成配置漂移。
  - **修复建议**：首次启动时生成一个 `seeded` 标记文件，或使用独立的初始化脚本替代运行时的自动 seed。

---

## 📋 修复优先级速查

| 优先级 | Issue | 关键词 |
|--------|-------|--------|
| P0 | 001, 002, 003, 019, 020, 021, 022 | 安全 / panic / DoS |
| P1 | 004, 005, 007, 008, 009, 024, 025, 047, 051, 052 | 架构 / 稳定性 / 性能 / 构建失败 |
| P2 | 010, 011, 012, 013, 014, 032, 037, 038, 040, 062, 064 | 逻辑 / 并发 / 资损风险 / 越权 |
| P3 | 015, 016, 017, 018, 023, 026, 027, 028, 029, 030, 031, 033, 034, 035, 036, 039, 041, 042, 043, 044, 045, 046, 048, 049, 050, 053, 054, 055, 056, 057, 058, 059, 060, 061, 063, 065, 066, 067, 068, 069, 070, 071, 072 | 代码质量 / 可维护性 / 工程化 / 用户体验 |
