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

- [x] ~~**ISSUE-051: `go.mod` 声明 `go 1.26.3`~~~~（已确认存在于 gvm，但需验证 Docker Hub 镜像是否存在）~~
  - 位置：`go.mod:3`
  - **修正说明**：经 `gvm listall` 确认 `go1.26.3` 确实存在。此前误判。但 Go 1.26.3 是否已发布官方稳定版仍需验证 Docker Hub `golang:1.26-alpine` 镜像是否存在——如果不存在，构建仍然失败。
  - **修复建议**：验证 `docker pull golang:1.26-alpine` 是否成功；若不存在则降级到已有稳定版镜像。

- [ ] **ISSUE-052: `go.mod` 中部分依赖标记为 `indirect` 但实际被直接 import，说明 `go.mod` 未被 `go mod tidy` 维护**
  - 位置：`go.mod:21`、`go.mod:30`
  - `gin-gonic/gin` 在 `server.go:18` 直接 import，`goccy/go-yaml` 在 `kube/client.go` 直接 import，但 `go.mod` 均标记为 `// indirect`；且存在 `go.yaml.in/yaml/v2`、`go.yaml.in/yaml/v3` 等非标准域名依赖。
  - **修复建议**：运行 `go mod tidy` 修复 direct/indirect 标记；清理非标准域名依赖。

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

- [ ] **ISSUE-073: 所有 Kubernetes `List` 调用都没有分页，大集群时可能 OOM**
  - 位置：`internal/kube/client.go:208-218`、`internal/kube/client.go:252`、`internal/kube/client.go:313-317`
  - `Deployments("").List(...)`、`StatefulSets("").List(...)`、`DaemonSets("").List(...)`、`RoleBindings("").List(...)`、`ClusterRoleBindings().List(...)`、`AppProjects.List(...)` 全部使用空的 `metav1.ListOptions{}`，没有设置 `Limit` 和 `Continue`，会一次性拉取全量数据到内存。在拥有数千 namespace、数万工作负载的集群中极易导致 OOM。
  - **修复建议**：在 `ListOptions` 中设置合理的 `Limit`（如 500），循环分页读取；或改用 [Informer](https://pkg.go.dev/k8s.io/client-go/informers) 进行流式处理。

- [ ] **ISSUE-074: `ArgoCDStatus` 使用 `if err == nil` 掩盖了所有 API 错误，可能返回不完整的状态**
  - 位置：`internal/kube/client.go:234-273`
  - `ArgoCDStatus` 中多次调用 K8s API（ConfigMap、Deployment、AppProject List、RulesForServiceAccount），每个调用失败后错误被静默忽略（`if err == nil { ... }`）。如果 cluster-admin 权限查询失败，返回的 `ApplicationControllerClusterAdmin = false` 会误导用户认为 controller 不危险。
  - **修复建议**：将错误收集到 `[]error` 并返回 `ArgoCDStatus` + `error`，在 UI 中以 warning 形式提示“部分状态检测失败”。

- [ ] **ISSUE-075: `/api/templates/render`、`/api/templates`、`/api/tool-profiles` 没有权限校验，任何人可访问**
  - 位置：`internal/app/server.go:54-57`
  - 尽管这些是只读端点，但 `handleRenderTemplate` 可触发 Go template 引擎执行任意用户传入的参数（`req.Params`），无身份校验意味着任何人都可以发起模板渲染请求（虽然写操作需通过 plan）。
  - **修复建议**：即使是只读端点，也至少要求 `currentUser()` 返回有效身份；`handleRenderTemplate` 的 params 输入做沙箱校验。

- [ ] **ISSUE-076: `handleApplyPlan` 和 `handleRollbackPlan` 在 API 层面没有幂等保护，可重复执行**
  - 位置：`internal/app/server.go:586-654`、`internal/app/server.go:660-689`
  - 即使前端按钮在 `status === 'applied'` 时 disabled，直接调用 `POST /api/plans/:id/apply` 仍可多次应用同一个 plan；已 rolled-back 的 plan 也可以再次 rollback（甚至再次 apply），导致集群状态不可预测。
  - **修复建议**：在 handler 入口处校验 `p.Status`，`applied` 的 plan 禁止再次 apply；`rolled-back` 的 plan 禁止再次 rollback；引入计划级别的乐观锁（如 `version` 字段）。

- [ ] **ISSUE-077: `RestoreSnapshots` 中快照 YAML 如果包含 `resourceVersion`/`uid`/`creationTimestamp` 等只读字段，apply 会失败**
  - 位置：`internal/kube/client.go:532-538`
  - `SnapshotObjects` 将完整对象 YAML（包含 K8s 系统注入的只读字段）原样记录；`RestoreSnapshots` 调用 `DecodeYAML` 后再 `ApplyYAML`，`metadata.resourceVersion`、`metadata.uid` 等字段会导致 Server-Side Apply 报错。
  - **修复建议**：快照前或恢复前，用 `unstructured.SetNestedField` 删除 `metadata.resourceVersion`、`metadata.uid`、`metadata.creationTimestamp`、`metadata.generation`、`metadata.managedFields` 等字段。

- [ ] **ISSUE-078: `ApplyYAML` 使用 `Force: true` 的 Server-Side Apply，会覆盖其他字段管理器的字段**
  - 位置：`internal/kube/client.go:386-388`
  - `ApplyOptions{FieldManager: "rbac-governance-console", Force: true}` 会强制接管所有字段所有权，如果其他字段管理器（如 `kubectl-client-side-apply` 或 GitOps 工具）已经拥有这些字段，将导致配置漂移和所有权冲突。
  - **修复建议**：对新建资源使用 `Force: false`，仅对明确由本系统管理的更新使用 `Force: true`；或在文档中明确警告此行为。

- [ ] **ISSUE-079: `plan.yaml` 和 `rollback.YAML` 存储完整对象 YAML，大 CRD 场景下 Plan JSON 可能膨胀到数 MB**
  - 位置：`internal/app/models.go:128`、`internal/app/models.go:133`、`internal/kube/client.go:450-454`
  - 每个 Plan 都存储渲染后的完整 YAML 字符串（数百行）；Rollback snapshot 中又存储每个资源的完整 YAML 字符串（通过 `json.NewYAMLSerializer` 序列化）。如果资源是 CRD 或包含大量字段，单个 Plan 的 JSON 可能膨胀到数 MB，state.json 无限增长。
  - **修复建议**：rollbacks 存储对象引用（`apiVersion`/`kind`/`namespace`/`name`）+ patch diff，而不是完整 YAML；或用外部对象存储（S3 / MinIO）。

- [ ] **ISSUE-080: `handleApplyPlan` 没有检查 plan 是否已经被 applied，重复 apply 无后端拦截**
  - 位置：`internal/app/server.go:586-589`
  - `handleApplyPlan` 只检查用户权限和 cluster 连接状态，没有检查 `plan.Status`；如果攻击者绕过前端重复调用 apply，会产生重复的 apply 操作和不可预测的集群状态。
  - **修复建议**：与 ISSUE-076 类似，在 API 层增加状态机校验：`planned` → `applied`，已 `applied` / `rolled-back` / `failed` 的 plan 禁止重新 apply。

- [ ] **ISSUE-081: `NewStore()` 中 `state.json` 文件损坏时仅 log 而不停止，导致数据静默丢失**
  - 位置：`internal/app/store.go:43-45`
  - `load()` 遇到 JSON 格式错误时只打印 `log.Printf("load state: %v", err)` 然后继续，所有历史数据（cluster、plan、audit）全部丢失，但进程继续运行。管理员可能长时间未察觉数据已清空。
  - **修复建议**：区分 "文件不存在"（正常首次启动）和 "文件损坏"（致命错误）；文件损坏时应 panic 或返回错误，阻止进程以空状态继续。

- [ ] **ISSUE-082: `handleCreateTemplate` 允许用户提交任意 YAML 字符串作为模板内容，未做语法校验和安全扫描**
  - 位置：`internal/app/server.go:415-432`
  - 用户提交的 `tmpl.Resources` 中 `Template` 字段可为任意 Go template 语法字符串；虽然 Go template 本身不执行命令，但自定义模板可能被恶意用于注入带有 `{{ .namespace }}` 等占位符的任意 K8s 资源，包括 `Secret`、`ConfigMap` 等敏感资源。
  - **修复建议**：对提交的模板 YAML 做 `kube.DecodeYAML` 预校验，确保能解析为合法 K8s 对象；限制自定义模板只能生成 `rbac.authorization.k8s.io`、`rbacmanager.reactiveops.io` 等白名单 API group。

- [ ] **ISSUE-083: `defaultAccessChecks` 只校验 namespace-scoped 资源，缺少 cluster-scoped 权限检查**
  - 位置：`internal/app/plan_validation.go:105-126`
  - validate 仅检查 `configmaps`、`services`、`deployments` 等 namespace-scoped 资源的 CRUD，但如果模板授予了 `ClusterRole`、`Namespace`、`Node` 等 cluster-scoped 权限，SAR 校验不会覆盖这些权限。
  - **修复建议**：根据模板 scope（`cluster` / `namespace` / `mixed`）动态生成 SAR checks，cluster-scoped 模板必须额外校验 cluster-level 资源访问。

- [ ] **ISSUE-084: `argoCDFindings` 对 Argo CD 版本为空或 v1.x 不做处理，可能产生错误结论**
  - 位置：`internal/app/analyzer.go:26-35`
  - 如果 `status.Version == ""`，版本检查条件 `strings.HasPrefix(..., "2.")` 不成立，不会添加检查提示，但空版本可能意味着检测失败，不应视为安全；v1.x 的 Argo CD 完全不支持 sync impersonation，代码没有处理这种边界。
  - **修复建议**：对 `status.Version == ""` 单独提示 "版本未知"；对 v1.x 添加明确的 "不支持 sync impersonation" finding。

- [ ] **ISSUE-085: `autoRegisterInCluster` 失败时完全静默，用户无法得知自动注册未发生**
  - 位置：`internal/app/incluster.go:11-35`
  - 如果 `kube.NewInCluster()` 失败（如不在集群中）、`Ping` 失败、已有 in-cluster 记录，函数都直接 `return`，没有日志也没有 frontend 状态反馈。用户可能疑惑为何页面上没有出现 `in-cluster`。
  - **修复建议**：将自动注册结果记录到结构化日志，并在 Health 或 `/api/me` 接口的响应中返回一个布尔字段 `autoInCluster`，让 UI 可选择性提示 "检测到集群内环境但自动注册失败"。

- [ ] **ISSUE-086: `HasRBACManager` 对错误类型判断不精确，`not found` 的字符串匹配可能漏判**
  - 位置：`internal/kube/client.go:189-205`
  - `strings.Contains(strings.ToLower(err.Error()), "not found")` 这种字符串匹配方式不可靠：不同版本的 K8s API server 返回的错误 message 可能不同（如 "the server could not find the requested resource"），导致漏判并返回 `false`。
  - **修复建议**：使用 `errors.Is(err, discovery.ErrGroupDiscoveryFailed)` 或检查 HTTP 状态码 `err.(*apierrors.StatusError).Status().Code == 404`，而不是字符串匹配。

- [ ] **ISSUE-087: `Plan.Status` 是裸字符串，缺乏编译期类型安全，易出现拼写错误**
  - 位置：`internal/app/models.go:131`
  - `Status string` 没有对应的常量枚举，代码中各处使用 `"planned"`、`"applied"`、`"failed"`、`"rolled-back"` 硬编码字符串，极易引入拼写错误导致状态机失效。
  - **修复建议**：定义常量 `type PlanStatus string` + `const StatusPlanned PlanStatus = "planned"` 等。

- [ ] **ISSUE-088: `SnapshotObjects` 使用 `json.NewYAMLSerializer` 生成 YAML，与原始资源的 field order 和格式可能不同**
  - 位置：`internal/kube/client.go:449-454`
  - `json.NewYAMLSerializer` 使用 JSON 标签的字母顺序序列化 YAML（如 `metadata` 字段在 `apiVersion` 之后），而用户原始 YAML 可能有不同的字段顺序。虽然不影响功能，但 diff 对比时会产生不必要的格式差异，增加评审难度。
  - **修复建议**：回滚时从 API server 获取当前资源重新序列化；或记录原始 YAML 字符串（从 server 返回的数据本身就是原始格式）。

- [ ] **ISSUE-089: API handler 混用 `gin.Context` 和标准库 `http.Request`/`ResponseWriter`，端起架子不匹配**
  - 位置：`internal/app/server.go:80-91`
  - `wrap` / `wrapWithID` 将 Gin handler 适配为标准库 handler，但 Gin 的版本（1.12）内部已经能支持这些功能；混用导致无法使用 Gin 的日志、绑定、验证等高级功能，并且需要手动 `c.Request.SetPathValue` 这种 hack。
  - **修复建议**：直接使用 `gin.HandlerFunc` 作为 handler 签名，完全拥抱 Gin 框架，或完全脱离 Gin 改用标准库 `net/http`。

- [ ] **ISSUE-090: `matchToolProfile` 中 `labelsMatch` 从未生效，因为所有内置 profile 的 `Labels` 字段都是 nil / 空**
  - 位置：`internal/app/tool_profiles.go:19-27`
  - `labelsMatch(ref.Labels, profile.Labels)` 会遍历 `profile.Labels`，但 `builtinToolProfiles` 中所有 profile 的 `Labels` 都是 nil / 空 map，因此该条件永远返回 `true`（循环不执行就是空集合等于 true），对匹配逻辑没有实际贡献。
  - **修复建议**：要么为非 argocd profile 补充 `Labels`（如 `app.kubernetes.io/name: argocd`），要么完全移除 `labelsMatch` 逻辑，只用 `textMatches`。

- [ ] **ISSUE-091: `Cluster.Kubeconfig` 在 `/api/clusters` 等 API 响应中明文返回，任何有集群查看权限的用户可获取完整 kubeconfig**
  - 位置：`internal/app/server.go:170-178`
  - `handleListClusters` 返回 `Cluster` 结构体完整 JSON，其中 `Kubeconfig` 字段为非空字符串时不会被 `omitempty` 省略。tenant-admin 只需被分配到该集群即可在响应中获得完整 kubeconfig，完全绕过平台管控直连集群。
  - **修复建议**：API 响应专用 DTO，显式排除 `Kubeconfig`；或增加 `json:"-"` tag，仅在需要时（如 `/api/clusters/:id/kubeconfig` 且平台 admin）单独暴露。

- [ ] **ISSUE-092: `handleImportCluster` 导入的 kubeconfig 未校验实际连接的集群是否与用户声明的名称一致**
  - 位置：`internal/app/server.go:187-222`
  - 用户可上传任意 kubeconfig（如生产集群的 kubeconfig）但命名为 "test-cluster"；系统仅测试连通性，不验证集群身份。后续所有操作都针对这个被错误标记的集群执行，造成管理混乱。
  - **修复建议**：连通性测试成功后，调用 `discovery` 获取 `ServerVersion` 和集群 UID（如 `kube-system` namespace 的 UID），与已有集群做去重校验；在 UI 中展示从 kubeconfig 解析出的 `current-context` 供用户确认。

- [ ] **ISSUE-093: `RestoreSnapshots` 在 `metadata.managedFields` 存在时，Server-Side Apply 会报字段所有权冲突**
  - 位置：`internal/kube/client.go:532-538`
  - `SnapshotObjects` 获取的 YAML 包含 `metadata.managedFields`（K8s server-side apply 字段管理元数据）。`RestoreSnapshots` 将其原样解码并 Apply，`managedFields` 会触发 "field is immutable" 或所有权冲突错误。
  - **修复建议**：快照生成或恢复前，从 `unstructured.Unstructured` 中删除 `metadata.managedFields`、`metadata.uid`、`metadata.resourceVersion`、`metadata.creationTimestamp`；或改用 ` unstructured.RemoveNestedField(existing.Object, "metadata", "managedFields")`。

- [ ] **ISSUE-094: `handleRollbackPlan` 未校验 plan 当前状态，未 applied 的 plan 也可回滚**
  - 位置：`internal/app/server.go:660-689`
  - `handleRollbackPlan` 仅检查 `len(p.Rollback) > 0`，不检查 `p.Status`。即使 plan 从未被 applied（`status: "planned"`），只要 snapshot 存在（apply 前的预检测），就可以调用 rollback，将集群恢复到一个从未被 plan 触及的状态——虽然无害但语义错误；更危险的是已 `rolled-back` 的 plan 可被再次回滚，可能把之后的人工修改也一并覆盖。
  - **修复建议**：状态机校验：`applied` 和 `failed` 允许回滚；`planned` 返回 409；已 `rolled-back` 的 plan 将 `Rollback` 置空或返回 409。

- [ ] **ISSUE-095: `newClient` 修改传入 `*rest.Config` 的 Timeout，对调用方产生副作用**
  - 位置：`internal/kube/client.go:139-148`
  - `config.Timeout = 20 * time.Second` 直接修改传入指针。若未来调用方复用同一 `rest.Config` 创建多个 client（如 informer + dynamic），前面的 client 会覆盖后面的 timeout 配置。
  - **修复建议**：先 `config2 := *config`（浅拷贝）再修改 `config2.Timeout`，或创建新的 `rest.CopyConfig(config)`。

- [ ] **ISSUE-096: K8s REST client 未配置 QPS/Burst，大规模操作会被 client-go 内部限流**
  - 位置：`internal/kube/client.go:139-148`
  - `rest.Config` 默认 `QPS = 5`、`Burst = 10`。在扫描或 apply 大量资源时（如 50 个 namespace、200 个 RoleBinding），client-go 会自动限速，导致 45 秒超时内无法完成操作。
  - **修复建议**：根据场景动态设置 `config.QPS`（如 50）和 `config.Burst`（如 100），或允许通过环境变量覆盖。

- [ ] **ISSUE-097: `api()` 前端函数没有超时、重试、网络错误降级，弱网环境体验极差**
  - 位置：`web/src/App.vue`
  - `fetch` 调用没有 `AbortController`，也没有任何超时处理；网络临时中断时请求永远挂起，用户看不到错误；`catch(() => ({}))` 吞掉了 JSON 解析错误，导致 `body.error` 为空时错误提示仅显示 "OK" 或无意义的状态文本。
  - **修复建议**：封装 `fetchWithTimeout(path, options, timeoutMs)`，使用 `AbortController.signal`；网络错误时显示明确的 "网络连接失败，请检查后端服务" 提示。

- [ ] **ISSUE-098: `handleCreatePlan` 和 `handleApplyPlan` 中的 `Cleanup` 逻辑在 apply 失败后不会自动回滚已应用的 docs**
  - 位置：`internal/app/server.go:630-653`
  - `ApplyYAML` 失败时，代码仅将 plan 标记为 `failed` 并返回错误。但此时 `Rollback` snapshot 已保存，如果部分 docs 已经 applied（apply 是 foreach 遍历，可能在第 N 个 doc 失败），前 N-1 个 doc 仍留在集群中，造成半应用状态；Cleanup 也未执行（因为它在 ApplyYAML 成功之后）。
  - **修复建议**：`ApplyYAML` 失败时，自动触发基于 `Rollback` 的恢复（但仅恢复已 applied 的部分），或在文档中明确告知用户 "apply 失败，请手动检查并回滚"。

- [ ] **ISSUE-099: `TenantCredentialResponse` 返回的 kubeconfig / token 没有审计日志记录**
  - 位置：`internal/app/server.go:349-392`
  - 虽然 `handleCreateTenantCredential` 调用了 `s.audit("tenant.credential.create", ...)`，但审计记录中不包含生成的 token 或 kubeconfig 的指纹（如 SHA256 hash），也无法追踪 token 是否被泄露或滥用。
  - **修复建议**：审计记录中增加 `tokenFingerprint`（SHA256 前 8 位）和 `clientIP`；如果业务允许，记录 token 的 `jti`（如果 K8s TokenRequest 支持）。

- [ ] **ISSUE-100: `builtin_templates.go` 中所有模板资源以 Go 常量字符串硬编码，无法运行时热更新或版本灰度**
  - 位置：`internal/app/builtin_templates.go`
  - 内置模板编译进 binary，任何模板修正都需要重新编译、重新发布镜像。运维团队无法在不发版的情况下修复一个高危权限模板（如 `argocd-control-plane` 的 rule 写错了）。
  - **修复建议**：内置模板从 ConfigMap 或外部配置源加载，支持运行时 reload；或至少支持在启动时从指定目录的 YAML 文件覆盖内置模板。

- [ ] **ISSUE-101: `saveLocked` 直接覆写 `state.json`，写入过程中进程崩溃会导致文件截断损坏**
  - 位置：`internal/app/store.go:359-376`
  - `os.WriteFile(s.path, b, 0o600)` 直接原地覆写，若进程在写入中途被 SIGKILL 或机器掉电，文件会被截断，所有持久化数据丢失。
  - **修复建议**：使用“写入临时文件 + `fsync` + `rename`”的原子写模式（如 `os.WriteFile(tmpPath)` + `os.Rename(tmpPath, s.path)`）。

- [ ] **ISSUE-102: 前端 `v-html` 渲染导航图标 SVG，存在潜在 XSS 注入面**
  - 位置：`web/src/App.vue:1094`
  - `navIcons` 对象中的 SVG 字符串通过 `v-html` 注入 DOM，虽然目前为硬编码，但该写法形成了不良示范；若未来 `navIcons` 数据来自接口，攻击者可直接注入脚本。
  - **修复建议**：将导航图标提取为独立的 Vue 单文件组件，或使用 `vue-dompurify-html` 等安全插件；彻底移除 `v-html`。

- [ ] **ISSUE-103: `refresh()` 使用 `Promise.all`，任一接口失败导致全部数据不更新**
  - 位置：`web/src/App.vue:525-545`
  - `Promise.all([clusters, templates, plans, audit])` 只要其中一个请求失败（如 `/api/audit-events` 超时），其余三个已返回的数据也会被 discard，页面整体空白或仅显示错误。
  - **修复建议**：改用 `Promise.allSettled`，每个接口独立处理 `fulfilled`/`rejected`，失败的接口提示局部错误，不阻塞其他数据渲染。

- [ ] **ISSUE-104: `applyPlan` 成功后强制跳转到 `tools` 视图，用户丧失当前上下文**
  - 位置：`web/src/App.vue:697-706`
  - 用户在 `plans` 页面点击 Apply，成功后 `state.view = 'tools'` 被强制切换；如果用户需要继续查看 plans 列表或进行 rollback，必须手动切回，体验差。
  - **修复建议**：apply 成功后保持在当前 `plans` 视图，仅通过 toast / 通知提示操作成功，让用户自主选择下一步操作。

- [ ] **ISSUE-105: `renderText` 每次调用都重新解析 `text/template`，无缓存，高频渲染性能差**
  - 位置：`internal/app/template_registry.go:68-78`
  - 每次模板预览或创建 plan 时，都执行 `template.New("resource").Parse(src)`；对于高频操作或批量渲染，重复解析带来显著 CPU 开销。
  - **修复建议**：在 `TemplateRegistry` 中维护 `map[string]*template.Template` 缓存，按 `templateID + resourceIndex` 缓存已解析模板；参数变化只需 `Execute`，无需重新 Parse。

- [ ] **ISSUE-106: `builtinToolProfiles` 与 `classify` 对 `log-collector` 的文本分隔不一致**
  - 位置：`internal/app/tool_profiles.go:15`、`internal/kube/client.go:294-310`
  - `builtinToolProfiles` 中 log-collector 的 `MatchText` 为逗号分隔（`"promtail,grafana-agent,alloy"`），而 `classify` 将名称、namespace、标签以空格拼接后做 `strings.Contains(text, token)`。`classify` 能正确匹配，因为 token 是 `promtail` 等不含逗号，但分隔符不一致增加了维护者的理解成本，且 `MatchText` 若未来包含空格分隔的多 token 容易产生歧义。
  - **修复建议**：统一 `classify` 的分隔符逻辑，或在 `textMatches` 中明确处理空格与逗号两种分隔；添加单元测试保证 profile 与 classify 的匹配一致性。

- [ ] **ISSUE-107: `handleListPlans` 列表接口返回完整 `YAML` 和 `Rollback` 字段，响应体失控**
  - 位置：`internal/app/server.go:481-483`、`internal/app/models.go:128,133`
  - `Plan` 结构体包含体积巨大的 `YAML` 字符串和 `Rollback` 快照，列表接口直接原样返回。当 plans 数量达到数百个时，响应体可能达到数十 MB，导致前端内存和带宽爆炸。
  - **修复建议**：列表接口返回 `PlanSummary`（不含 `YAML`、`Rollback`），详情接口 `/api/plans/:id` 返回完整 `Plan`。

- [ ] **ISSUE-108: `ComputeGovernanceState` 遍历全量 plans 判断 in-progress，时间复杂度 O(N*M)**
  - 位置：`internal/app/analyzer.go:113-130`
  - 每个 tool 都需要遍历所有 plans 查找匹配项。当集群数量多、plans 历史量大时，每轮 scan 的性能呈平方级下降。
  - **修复建议**：在后端维护 `map[toolID+clusterID][]Plan` 索引，或在创建/更新 plan 时直接更新对应 tool 的 governanceState，避免全量遍历。

- [ ] **ISSUE-109: `state.json` 使用相对路径，工作目录不确定时数据文件位置不可预期**
  - 位置：`internal/app/store.go:29-32`
  - 默认路径为 `data/state.json`，若 server 通过 systemd、supervisor 或不同 shell 脚本启动，工作目录可能不是项目根目录，导致数据文件在不可见位置生成或找不到。
  - **修复建议**：默认使用绝对路径（如 `~/.config/rbac-governance/state.json` 或 `/data/state.json`），或在启动时校验并打印实际使用的绝对路径。

- [ ] **ISSUE-110: `deploy/kubernetes.yaml` 使用 `latest` 镜像标签且来自私有仓库，无版本控制**
  - 位置：`deploy/kubernetes.yaml:76`
  - 镜像指定为 `dockerhub.kubekey.local/library/rbac-governance:latest`，`latest` 标签无固定版本语义，且配合 `imagePullPolicy: IfNotPresent`，节点上缓存的旧镜像可能长期不更新，导致滚动更新行为不可预期。
  - **修复建议**：使用语义化版本 tag（如 `v0.1.0-sha`），CI/CD 流水线自动替换 yaml 中的 tag；或将 `imagePullPolicy` 改为 `Always` 并在部署时显式更新镜像版本。

- [ ] **ISSUE-111: `inferTemplateParams` 正则无法匹配嵌套字段、管道语法和 `with`/`range` 块内变量**
  - 位置：`web/src/App.vue:805-814`
  - 正则 `/\{\{\s*(?:dns\s+)?\.([A-Za-z][A-Za-z0-9_]*)\s*\}\}/g` 仅能识别简单 `{{ .xxx }}`，对于 `{{ .Values.namespace }}`、`{{ .foo | dns }}`、`{{ with .bar }}{{ .baz }}{{ end }}` 等 Go template 语法均无法提取，导致自定义模板参数推断不全。
  - **修复建议**：简化参数推断为“手动填写 + 后端预渲染校验”，或引入兼容 Go template AST 的解析器（如 wasm 执行 `text/template/parse`）准确提取字段。

- [ ] **ISSUE-112: `tenantCredentialOutput` 在前端内存中长期保留，无自动清理机制**
  - 位置：`web/src/App.vue:640-655`、`web/src/App.vue:675-690`
  - 生成的 kubeconfig / token 被写入 `state.tenantCredentialOutput`，只要用户不刷新页面，该敏感内容一直在 Vue reactive 中，浏览器 DevTools 可直接查看和复制。
  - **修复建议**：设置定时器在展示 60 秒后自动清空 `tenantCredentialOutput`；或提供醒目“已复制，清除”按钮，提交后立即擦除内存。核心修复是根本不在前端存储完整凭证，改用一次性下载链接。

- [ ] **ISSUE-113: `handleRollbackPlan` 恢复 snapshot 后未校验实际集群状态是否与预期一致**
  - 位置：`internal/app/server.go:660-694`
  - `RestoreSnapshots` 后若 K8s API server 返回成功但实际对象未被恢复（如因 webhook 拒绝），plan 仍被标记为 `rolled-back`，用户误以为回滚完成。
  - **修复建议**：rollback 后按 `Rollback` 列表逐条 Get 校验对象存在性/YAML 匹配度，不匹配的标记为 `rolled-back-partial` 并给出明细。

- [ ] **ISSUE-114: `frontendDist()` 运行时未校验静态文件目录是否可用，缺失时启动不报错但前端 404**
  - 位置：`internal/app/server.go:101-116`
  - 若 `FRONTEND_DIST` 未设置，且 `repoRoot()` / `os.Executable()` 路径下均找不到 `web/dist/index.html`，`frontendDist()` 仍返回一个无效路径；`httpServer` 会正常启动，但用户访问时持续收到 404 或 NoRoute 的 fallback index.html 缺失页面。
  - **修复建议**：`NewServer()` 中调用 `frontendDist()` 后显式检查 `index.html` 是否存在，不存在则 `log.Fatal` 退出并给出明确错误。

- [ ] **ISSUE-115: `DiscoveryWorkloads` 仅扫描 Deployment/StatefulSet/DaemonSet，遗漏 Job/CronJob/ReplicaSet 等工作负载**
  - 位置：`internal/kube/client.go:207-231`
  - 部分工具以 Job 或 CronJob 形式运行（如一次性数据迁移任务、定时扫描器），或因 ReplicaSet 直接管理 Pod（无 Deployment）时无法被识别，导致治理盲区。
  - **修复建议**：补充 `BatchV1().Jobs()`、`BatchV1().CronJobs()`、`AppsV1().ReplicaSets()` 的发现逻辑；或统一使用 serviceaccount 反查绑定的方式发现权限主体，而非仅遍历 workload 资源。

- [ ] **ISSUE-116: `handleCreateTenantCredential` 未预校验目标 ServiceAccount 是否存在于目标命名空间**
  - 位置：`internal/app/server.go:349-397`
  - 代码直接调用 `client.CreateServiceAccountToken(ctx, req.Namespace, req.ServiceAccount, ...)`，若 SA 不存在，K8s 会返回报错；但 API 层返回的是 502 Bad Gateway，用户无法区分是连接失败还是 SA 不存在。
  - **修复建议**：生成 token 前先 `Get` ServiceAccount，不存在时返回明确的 404/message，提升可观测性。

- [ ] **ISSUE-117: 前端 `run()` 辅助函数统一吞掉所有错误，未区分 HTTP 状态码类别**
  - 位置：`web/src/App.vue:855-863`
  - 401/403/500/网络错误全部被 catch 后以同一方式显示 `state.error = 错误消息`，用户无法得知是需要重新登录、无权限还是服务端故障。
  - **修复建议**：在 `api()` 中保留 `response.status`，`setError` 根据状态码展示不同提示；401 时清空 `state.me` 并提示登录失效，403 时提示无权操作，网络错误时提示检查后端连接。

- [ ] **ISSUE-118: `ArgoCDStatus` 的 version 解析无法处理 `@sha256` 格式或无 tag 的镜像**
  - 位置：`internal/kube/client.go:240-247`
  - 代码按 `:` split image 字符串提取 tag，若 image 为 `argocd@sha256:abc123`，提取出的 "version" 是 sha256 摘要而非版本号；若 image 无 tag 则 Version 为空。两者都会导致后续版本检查逻辑（`HasPrefix(..., "2.")`）失效或产生误判。
  - **修复建议**：使用正则 `[^:]+[:@]?(.+)?$` 或容器镜像解析库提取 tag/digest；对空 version 添加明确的 "unknown" 标记。

- [ ] **ISSUE-119: `handleCreatePlan` 中的 `Cleanup` 列表依赖最近一次 scan 的 findings，数据可能已过期**
  - 位置：`internal/app/server.go:517-523`
  - `cleanupBindingRefs(tool.Findings)` 直接读取 store 中缓存的 findings。如果 scan 是在几小时甚至几天前执行的，期间集群权限可能已经发生变化（如管理员手动增删了 binding），plan 的 cleanup 列表可能漏删或误删。
  - **修复建议**：创建 plan 时若 `tool.UpdatedAt` 超过阈值（如 10 分钟），提示用户“数据已过期，请先重新 scan”；或在 apply 前实时重新查询 bindings 做二次确认。

- [ ] **ISSUE-120: `ApplyYAML` 按 doc 顺序逐个 apply，中间失败后无自动回滚，造成半应用状态**
  - 位置：`internal/kube/client.go:374-395`
  - `ApplyYAML` 遍历 docs 逐一 Server-Side Apply，若第 N 个 doc 失败，前 N-1 个 doc 已生效但 plan 标记为 `failed`，且不会自动触发 rollback，集群处于不一致状态。
  - **修复建议**：在 `handleApplyPlan` 的 `ApplyYAML` 失败分支中，基于已保存的 `Rollback` snapshot 执行自动恢复（仅恢复已成功 apply 的部分），或至少在前端提示 “部分资源已应用，请手动检查并回滚”。

- [ ] **ISSUE-121: `GovernancePanel` 的模板选择器和参数输入框未通过事件向上同步新值，用户无法切换模板或修改参数**
  - 位置：`web/src/components/GovernancePanel.vue:72-84`、`web/src/App.vue:1152-1168`
  - `select` 的 `@change` 和 `input` 的 `@input` 仅无差别地 `emit('template-change')`，既不传递新选中的 `templateId`，也不传递输入框的新值；父组件 `App.vue` 只绑定 `@template-change="onTemplateChange"`，没有 `@update:selectedTemplateId` 或 `@update:params`。这导致 GovernancePanel 的交互控件实际上处于只读状态，用户无法真正切换模板或填写参数。
  - **修复建议**：GovernancePanel 中 `select` 的 `@change` 应传递 `($event.target as HTMLSelectElement).value`；`input` 的 `@input` 应传递 `($event.target as HTMLInputElement).value`。父组件补充 `@update:selectedTemplateId` 和 `@update:params` 事件处理，更新 `state.selectedTemplateId` 和 `params`。

- [ ] **ISSUE-122: `handleCreatePlan` 未校验 ToolID 对应的 tool 是否属于请求中的 ClusterID**
  - 位置：`internal/app/server.go:496-498`
  - 代码仅校验 `req.ToolID != "" && !ok`（tool 是否存在），没有检查 `tool.ClusterID == req.ClusterID`。攻击者可以构造 `toolId=A_cluster` + `clusterId=B_cluster` 的 plan，将 A 集群扫描到的 tool 的 cleanup binding 误应用到 B 集群，造成 A 集群的 findings 被用于 B 集群的权限清理。
  - **修复建议**：获取 tool 后校验 `tool.ClusterID == req.ClusterID`，不匹配时返回 400。

- [ ] **ISSUE-123: `handleApplyPlan` 对 `RBACDefinition` 的检测使用字符串匹配而非结构化校验**
  - 位置：`internal/app/server.go:609-611`
  - `strings.Contains(p.YAML, "kind: RBACDefinition")` 在 YAML 注释中也可能出现该字符串，导致误拦截合法 plan；如果模板资源名中包含该子串同样会触发误报。
  - **修复建议**：调用 `kube.DecodeYAML(p.YAML)` 后遍历 unstructured objects，检查 `GetKind() == "RBACDefinition"`；或至少确保字符串匹配位于行首且紧跟冒号。

- [ ] **ISSUE-124: `renderText` 中 Go template 缺少参数时输出 `<no value>` 而非报错，生成无效 YAML**
  - 位置：`internal/app/template_registry.go:68-78`
  - Go template 对缺失字段默认输出字符串 `<no value>`。如果用户未填写某个参数或模板使用了未声明的字段，生成的 YAML 会出现 `name: <no value>` 这类无效内容，导致 apply 失败或产生难以排查的集群状态。
  - **修复建议**：在 `template.New(...).Option("missingkey=error")` 配置缺失字段报错；或在 `Render()` 后对输出结果做正则校验，禁止包含 `<no value>`。

- [ ] **ISSUE-125: `Dockerfile` 使用 `node:26-alpine` 镜像，Node.js 26 不存在，构建必定失败**
  - 位置：`Dockerfile:1`
  - Node.js 当前最新 LTS 为 v22，公开 release 最高为 v23。`node:26-alpine` 在 Docker Hub 上不存在，导致多阶段构建在第一阶段即失败。
  - **修复建议**：降级为实际存在的版本（如 `node:22-alpine`），并在 CI/CD 中验证镜像可用性。

- [ ] **ISSUE-126: 所有 `RBACDefinition` 模板硬编码 `rbacmanager.reactiveops.io/v1beta1`，与集群实际 CRD group 可能不匹配**
  - 位置：`internal/app/builtin_templates.go`
  - `HasRBACManager` 会探测 `rbacmanager.dev`、`rbacmanager.reactiveops.io`、`rbac-manager.reactiveops.io` 三个 group，但模板 YAML 中写死了 `rbacmanager.reactiveops.io/v1beta1`。如果集群安装的是其他 group 的 CRD，apply 会报 `No match for kind "RBACDefinition"`。
  - **修复建议**：在导入集群时将检测到的实际 CRD group/version 存入 `Cluster` 结构体，渲染模板时动态替换 `apiVersion`。

- [ ] **ISSUE-127: `tenantKubeconfig` 对 token 未做 YAML 安全转义，特殊字符会破坏 kubeconfig 格式**
  - 位置：`internal/app/server.go:787-813`
  - token 通过 `%s` 直接拼接进 YAML 字符串。如果 K8s 返回的 token 包含 `"`、`\n`、`: ` 等特殊字符（虽然通常不会，但在某些 token provider 场景下可能），生成的 kubeconfig 会成为非法 YAML，甚至可能被解析为其他字段。
  - **修复建议**：使用结构化生成（如 `goccy/go-yaml` 的 `yaml.Marshal`）或至少对 token 做 YAML 字符串转义。

- [ ] **ISSUE-128: `handleCreateUser` 未校验 role 合法性，可创建任意 role 字符串**
  - 位置：`internal/app/server.go:152-167`
  - `user.Role` 只检查了非空，没有校验是否为预定义角色之一（`platform-admin`、`tenant-admin`、`viewer`、`auditor`）。创建 `role: cluster-admin` 后，`canApply`/`canAdmin` 全部是 false，用户什么都做不了；但若前端或其他逻辑误将该值当作有效角色处理，会产生不可预期的授权结果。
  - **修复建议**：增加白名单校验：`user.Role` 必须在预定义角色集合中，否则返回 400。

- [ ] **ISSUE-129: `handleCreateUser` 未校验 tenantIds 是否存在于 store 中**
  - 位置：`internal/app/server.go:152-167`
  - `user.TenantIDs` 可直接传入任意不存在的 tenant ID（如 `"fake-tenant"`）。后续 `GetUser` 调用时这些 tenant 被忽略，导致用户实际无任何 namespace 权限，但状态显示正常，增加排障难度。
  - **修复建议**：创建 user 前遍历 `tenantIds`，校验每个 ID 在 `s.store.ListTenants()` 中存在，不存在的返回 400 并提示具体缺失项。

- [ ] **ISSUE-130: `dns1123` 未限制输出长度，超长输入可能导致 K8s 资源名超限**
  - 位置：`internal/app/template_registry.go:82-90`
  - `dns1123` 将输入全部转为小写并替换非法字符，但没有截断逻辑。若用户输入超长 namespace 或 serviceAccount 名（如 200 字符），输出可能超过 K8s 对资源名（63 字符）或 label value（63 字符）的长度限制，导致 apply 失败。
  - **修复建议**：截断至 63 字符并在尾部添加哈希摘要，确保唯一性和合规性。

- [ ] **ISSUE-131: 非 admin 用户 scan 也会更新 `Cluster.LastScanAt`，误导 admin 对数据完整性的判断**
  - 位置：`internal/app/server.go:343-344`
  - tenant-admin 或 viewer 执行 scan 时，`LastScanAt` 被更新为当前时间。admin 在 UI 上看到 "Last scan: 刚刚"，可能以为数据是全量最新的，但实际上 store 中只包含该 tenant 有权限的 namespace 的 tools。
  - **修复建议**：只有 `RolePlatformAdmin` 执行 scan 时才更新 `LastScanAt`，或在 `Cluster` 中拆分 `lastFullScanAt` / `lastPartialScanAt` 两个字段。

- [ ] **ISSUE-132: `httpError` 对所有错误返回原始错误字符串，可能泄露内部实现细节**
  - 位置：`internal/app/server.go:827-828`
  - `writeJSON(w, status, map[string]string{"error": err.Error()})` 将后端异常（如 `open /data/state.json: permission denied`、`dial tcp 10.0.0.1:443: i/o timeout`）直接暴露给客户端，攻击者可据此推断文件路径、网络拓扑或内部依赖。
  - **修复建议**：生产环境下对非 4xx 错误隐藏详情，返回统一消息 `"internal server error"`；将原始错误写入结构化日志供排查。

- [ ] **ISSUE-133: `tenantKubeconfig` 未处理 `apiServer` 为空字符串的情况，生成无效 kubeconfig**
  - 位置：`internal/app/server.go:787-813`
  - 如果 cluster.APIServer 为空（如 in-cluster 场景或异常导入），`server: %s` 会被替换为 `server: `（无值），生成的 kubeconfig 无法使用。
  - **修复建议**：在 `tenantKubeconfig` 入口处，若 `apiServer` 为空则返回明确的错误，或降级为 `https://kubernetes.default.svc`。

- [ ] **ISSUE-134: `seedDefaults` 每次启动无条件覆盖 builtin profiles，对 state.json 的手动修改在重启后丢失**
  - 位置：`internal/app/store.go:50-63`
  - 如果管理员出于某种原因直接编辑了 `state.json` 中的 builtin profile（如调整 `MatchText`），重启后 `seedDefaults` 会从 `builtinToolProfiles()` 重新覆盖，修改丢失。
  - **修复建议**：仅在 profile 不存在时才插入 builtin，不再覆盖已有 builtin profile；或提供显式的 "reset builtins" 管理操作。

- [ ] **ISSUE-135: `TenantCredentialGenerator.vue` 中 `credentialExpiration` 输入未校验 `NaN`，可能向后端发送非法 JSON number**
  - 位置：`web/src/components/TenantCredentialGenerator.vue:38`
  - `Number(($event.target as HTMLInputElement).value)` 在输入非数字（如字母）时会产生 `NaN`。浏览器 JSON.stringify 后变为 `null`，后端 `int64` 字段收到 `0`，与最小值校验 `<= 0` 触发默认 8 小时，前端输入与后端行为不一致。
  - **修复建议**：在 `@input` 中增加 `isNaN` 校验，非法输入时保持原值或清空，并给出提示。

- [ ] **ISSUE-136: `ClusterSelector.vue` 的 `change` 事件未传递选中值，组件 API 不完整**
  - 位置：`web/src/components/ClusterSelector.vue:17`
  - `select` 的 `@change` 仅触发 `emit('change')`，没有传递 `($event.target as HTMLSelectElement).value`。虽然当前父组件的 `onClusterChange` 不依赖事件参数，但该组件作为独立模块不具备通用性，且容易误导开发者。
  - **修复建议**：传递选中值 `emit('change', ($event.target as HTMLSelectElement).value)`；父组件若需兼容可不使用参数。

---

## 📋 修复优先级速查

| 优先级 | Issue | 关键词 |
|--------|-------|--------|
| P0 | 001, 002, 003, 019, 020, 021, 022 | 安全 / panic / DoS |
| P1 | 004, 005, 007, 008, 009, 024, 025, 047, 051, 052, 073, 091, 095, 096, 100, 101, 107, 109, 110, 125 | 架构 / 稳定性 / 性能 / 构建失败 / OOM |
| P2 | 010, 011, 012, 013, 014, 032, 037, 038, 040, 062, 064, 074, 075, 076, 077, 078, 080, 081, 083, 084, 086, 090, 092, 093, 094, 097, 098, 103, 106, 108, 113, 115, 116, 119, 120, 121, 122, 124, 126, 127, 128, 129, 131, 132 | 逻辑 / 并发 / 资损风险 / 越权 / SSA / 数据丢失 |
| P3 | 015, 016, 017, 018, 023, 026, 027, 028, 029, 030, 031, 033, 034, 035, 036, 039, 041, 042, 043, 044, 045, 046, 048, 049, 050, 053, 054, 055, 056, 057, 058, 059, 060, 061, 063, 065, 066, 067, 068, 069, 070, 071, 072, 079, 082, 085, 087, 088, 089, 099, 102, 104, 105, 111, 112, 114, 117, 118, 123, 130, 133, 134, 135, 136 | 代码质量 / 可维护性 / 工程化 / 用户体验 |
