<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import ClusterPanel from './components/ClusterPanel.vue'
import ToolList from './components/ToolList.vue'
import GovernancePanel from './components/GovernancePanel.vue'
import TenantPanel from './components/TenantPanel.vue'
import TemplatePanel from './components/TemplatePanel.vue'
import MyPermissions from './components/MyPermissions.vue'
import RequestForm from './components/PermissionRequest.vue'
import PermissionAdmin from './components/PermissionAdmin.vue'
import type { PermissionRequest } from './components/MyPermissions.vue'

export type Cluster = { id: string; name: string; context: string; apiServer: string; status: string; message: string; rbacManagerStatus: string; lastScanAt?: string }
export type Finding = { id: string; severity: 'high' | 'medium' | 'low'; title: string; description: string; resource: string; ruleId: string }
export type Tool = { id: string; clusterId: string; type: string; name: string; namespace: string; kind: string; serviceAccount: string; labels?: Record<string, string>; findings: Finding[]; recommendedTemplateIds: string[]; governanceState?: string; baselineMatched?: boolean }
export type Template = { id: string; tool: string; name: string; description: string; scope: string; riskLevel: 'high' | 'medium' | 'low'; builtin: boolean; params: Array<{ name: string; label: string; required: boolean; default?: string; description?: string }>; resources: Array<{ kind: string; template: string }> }
export type ValidationCheck = { allowed: boolean; namespace: string; verb: string; group: string; resource: string; name?: string; reason?: string; serviceAccount: string }
export type ResourceSnapshot = { apiVersion: string; kind: string; namespace?: string; name: string; yaml?: string; exists: boolean }
export type Plan = { id: string; clusterId: string; toolId: string; templateId: string; yaml: string; warnings: string[]; cleanup?: ResourceSnapshot[]; status: string; validation?: ValidationCheck[]; rollback?: ResourceSnapshot[]; createdAt: string; result: string }
export type AuditEvent = { id: string; action: string; clusterId: string; status: string; message: string; createdAt: string }
export type Tenant = { id: string; name: string; clusterIds: string[]; namespaces: string[] }
export type User = { id: string; name: string; role: string; tenantIds: string[] }
export type Me = { id: string; name: string; role: string; tenants: Tenant[] }
export type ToolProfile = { id: string; type: string; name: string; matchText: string; recommendedTemplateIds: string[]; builtin: boolean }
export type TenantCredential = { clusterId: string; namespace: string; serviceAccount: string; expirationSeconds: number; expiresAt?: string; token?: string; kubeconfig?: string }
export type Lang = 'zh' | 'en'

const messages = {
  en: {
    brand: 'RBAC Governance',
    views: { clusters: 'Clusters', tools: 'Tools', tenants: 'Tenants', templates: 'Templates', plans: 'Plans', audit: 'Audit', 'my-permissions': 'My Permissions', 'request-permission': 'Request', 'approval-queue': 'Approval' },
    subtitles: {
      clusters: 'Import clusters, test connectivity, and run permission scans.',
      tools: 'Inspect detected tools, ServiceAccounts, and risky Kubernetes RBAC findings.',
      tenants: 'Create tenant sync ServiceAccounts, AppProjects, and namespace bindings separately from tool control-plane permissions.',
      templates: 'Review built-in permission templates. Built-ins are versioned with the codebase.',
      plans: 'Review generated change plans and apply them to clusters after confirmation.',
      audit: 'Trace cluster imports, scans, plan creation, apply operations, and rollbacks.',
      'my-permissions': 'Review your current namespace and tool access, and request history.',
      'request-permission': 'Apply for new namespace or tool permissions.',
      'approval-queue': 'Approve, reject, or revoke permission requests.',
    },
    refresh: 'Refresh',
    currentUser: 'Current user',
    user: 'User',
    role: 'Role',
    tenantName: 'Tenant name',
    tenantScope: 'Tenant scope',
    tenantClusters: 'Clusters',
    tenantNamespaces: 'Namespaces',
    tenantUsers: 'Users',
    createTenant: 'Create tenant',
    createUser: 'Create user',
    tenantGovernance: 'Tenant governance',
    tenantGovernanceHelp: 'Use this page after the Argo CD control-plane baseline. It creates tenant ServiceAccounts and AppProjects explicitly; nothing is created by default for a new Argo CD.',
    tenantTemplate: 'Tenant template',
    tenantServiceAccount: 'Tenant ID',
    tenantServiceAccountHint: 'This name will be created as a ServiceAccount in the ArgoCD namespace',
    businessNamespace: 'Business Namespace',
    businessNamespaceHint: 'The target namespace this tenant will manage',
    namespacePattern: 'Namespace Match Rule',
    namespacePatternHint: 'All namespaces matching this rule will automatically be granted access',
    tenantLabelKey: 'Namespace label key',
    tenantLabelValue: 'Namespace label value',
    sourceRepo: 'Allowed source repository',
    adminGroup: 'Tenant Admin Group',
    adminGroupHint: 'Users in this group will have sync/get permissions on Argo CD Applications in this project.',
    explicitTenantRequired: 'Select a tenant template and fill tenant, SA, and namespace scope explicitly.',
    tenantControllerHint: 'The Argo CD namespace and controller SA are detected from the scanned application-controller workload.',
    tenantSaLocationHint: 'Tenant SA will be created in the Argo CD namespace ({namespace}) for centralized management.',
    tabLabels: { governance: 'Tenant Governance', credential: 'Credential Generator', scope: 'Tenant Management' },
    governanceState: { secured: 'Secured', 'needs-action': 'Needs Action', 'in-progress': 'In Progress' },
    securedMessage: 'Permissions converged to baseline.',
    clusterOverview: 'Cluster overview',
    importCluster: 'Import cluster',
    name: 'Name',
    kubeconfig: 'Kubeconfig',
    importAndTest: 'Import and test',
    useInCluster: 'Use current in-cluster',
    knownClusters: 'Known clusters',
    context: 'Context',
    apiServer: 'API Server',
    lastScan: 'Last scan',
    message: 'Message',
    openTools: 'Open tools',
    test: 'Test',
    scan: 'Scan',
    clusterCount: 'clusters',
    countSuffix: '',
    noClusters: 'No clusters imported yet.',
    cluster: 'Cluster',
    scanSelected: 'Scan selected cluster',
    detectedTools: 'Detected tools',
    namespace: 'Namespace',
    kind: 'Kind',
    serviceAccount: 'ServiceAccount',
    govern: 'Govern',
    noTools: 'No tools found. Run a scan first.',
    registerTool: 'Register tool profile',
    addTool: 'Add tool',
    close: 'Close',
    preview: 'Preview',
    resources: 'Resources',
    toolRegistry: 'Tool registry',
    toolRegistryHelp: 'Register new tool signatures here. Scans will use them to classify workloads in any namespace.',
    matchText: 'Match text',
    recommendedTemplates: 'Recommended template IDs',
    governanceAction: 'Governance action',
    tool: 'Tool',
    template: 'Template',
    targetNamespace: 'Target namespace',
    targetServiceAccount: 'Target ServiceAccount',
    templateParameters: 'Template parameters',
    previewYaml: 'Preview YAML',
    createPlan: 'Create plan',
    warning: 'Warning',
    currentPermissionCheck: 'Current permission check',
    proposedYaml: 'Proposed YAML',
    currentPermissionHelp: 'These checks show what the ServiceAccount can do before applying the plan.',
    proposedYamlHelp: 'This is the new Role / ClusterRole / RBACDefinition YAML that will be applied.',
    cleanupOldBindings: 'Clean up old risky bindings after apply',
    cleanupOldBindingsHelp: 'Apply the scoped template first, then remove the risky RoleBinding or ClusterRoleBinding detected in this scan.',
    cleanupBindings: 'Existing risky bindings to remove',
    cleanupCandidates: 'Existing risky bindings detected',
    cleanupCandidateHelp: 'These are existing risky bindings found by the latest scan. They are not new bindings. They are removed only when cleanup is enabled in the plan.',
    noCleanupBindings: 'No risky bindings selected for cleanup.',
    planNote: 'Current permission checks and proposed YAML are different by design. The checks show existing access; the YAML shows the change to be applied.',
    previewPlaceholder: 'Preview output will appear here.',
    selectTool: 'Select a tool to preview and apply a template. Argo CD tool governance only handles the control-plane baseline here.',
    noTemplatesForTool: 'No ready-made template is available for this component.',
    quickCredential: 'Generate credential for this SA',
    id: 'ID',
    scope: 'Scope',
    source: 'Source',
    builtIn: 'built-in',
    custom: 'custom',
    createTemplate: 'Create custom role template',
    templateCatalog: 'Template catalog',
    templateCatalogHelp: 'Built-in templates are stored in the codebase and rendered only when previewing or creating a plan.',
    builtinTemplates: 'Built-in templates',
    customTemplates: 'Custom templates',
    noCustomTemplates: 'No custom templates yet.',
    templatePreviewHelp: 'Preview the template source stored in the catalog. Built-ins are read-only; create a custom template when a tool needs different permissions.',
    customTemplateHelp: 'Use Go template placeholders such as {{ .namespace }} and {{ .serviceAccount }}. Parameters are detected automatically.',
    noParams: 'No parameters required.',
    requiredParam: 'required',
    optionalParam: 'optional',
    defaultValue: 'Default',
    risk: 'Risk',
    profile: 'Profile',
    permissionProfiles: { view: 'view', deploy: 'deploy', admin: 'admin', breakglass: 'breakglass' },
    tenantCredential: 'Tenant credential',
    credentialHelp: 'Generate a short-lived token or kubeconfig for a tenant ServiceAccount. The secret is returned once and is not stored.',
    credentialNamespace: 'Credential namespace',
    credentialServiceAccount: 'Credential ServiceAccount',
    credentialExpiration: 'Expiration seconds',
    credentialFormat: 'Format',
    generateCredential: 'Generate credential',
    credentialOutput: 'Credential output',
    credentialExpires: 'Expires at',
    params: 'Parameters',
    templateId: 'Template ID',
    templateYaml: 'Template YAML resources',
    created: 'Created',
    result: 'Result',
    validate: 'Validate',
    apply: 'Apply',
    rollback: 'Rollback',
    noPlans: 'No plans created yet.',
    time: 'Time',
    action: 'Action',
    status: 'Status',
    allowed: 'allowed',
    denied: 'denied',
    error: 'Error',
    confirmApply: 'Apply this plan to the selected cluster? Review the YAML before continuing.',
    confirmRollback: 'Rollback this plan using the saved snapshot?',
    severity: { high: 'high', medium: 'medium', low: 'low' },
    statusText: { connected: 'connected', error: 'error', installed: 'installed', missing: 'missing', planned: 'planned', applied: 'applied', failed: 'failed', 'rolled-back': 'rolled back', success: 'success', unknown: 'unknown' },
    scopeText: { namespace: 'namespace-scoped', cluster: 'cluster-scoped', mixed: 'mixed' },
    auditAction: {
      'tenant.create': 'tenant.create',
      'user.create': 'user.create',
      'cluster.import': 'cluster.import',
      'cluster.import_in_cluster': 'cluster.import_in_cluster',
      'cluster.auto_in_cluster': 'cluster.auto_in_cluster',
      'cluster.test': 'cluster.test',
      'cluster.scan': 'cluster.scan',
      'template.create': 'template.create',
      'tool_profile.create': 'tool_profile.create',
      'tenant.credential.create': 'tenant.credential.create',
      'plan.create': 'plan.create',
      'plan.validate': 'plan.validate',
      'plan.apply': 'plan.apply',
      'plan.rollback': 'plan.rollback',
    },
    findingTitle: {
      'cluster-admin': 'ServiceAccount is bound to cluster-admin',
      'wildcard-rbac': 'Wildcard RBAC permission',
      'read-only-wildcard-rbac': 'Broad read-only RBAC permission',
      'cluster-write': 'Cluster-wide write permission',
      'secret-read': 'Secret read access',
      'pod-exec': 'Pod exec permission',
      'privilege-escalation': 'Privilege escalation verb',
      'argocd-tenant-impersonation': 'Argo CD tenant impersonation binding',
      'no-high-risk-rbac': 'No high-risk RBAC detected',
      'argocd-controller-cluster-admin': 'Argo CD controller has cluster-admin',
      'argocd-sync-impersonation-disabled': 'Sync impersonation is not enabled',
      'argocd-no-destination-serviceaccounts': 'No AppProjects use destinationServiceAccounts',
      'argocd-version-check': 'Confirm Argo CD sync impersonation support',
    },
    findingDesc: {
      'cluster-admin': 'This grants unrestricted cluster access and should be replaced by a scoped template.',
      'wildcard-rbac': 'Wildcard verbs, resources, or API groups make the effective permission hard to audit.',
      'read-only-wildcard-rbac': 'The binding can read a broad set of resources but does not grant write or escalation verbs.',
      'cluster-write': 'The binding grants write permissions beyond a single namespace.',
      'secret-read': 'The ServiceAccount can read Kubernetes Secrets. Confirm this is required.',
      'pod-exec': 'Pod exec can be used to access workloads and mounted credentials.',
      'privilege-escalation': 'The ServiceAccount can bind, escalate, or impersonate privileges.',
      'argocd-tenant-impersonation': 'This is a tenant sync impersonation binding managed from the Tenants page, not a tool control-plane cleanup candidate.',
      'no-high-risk-rbac': 'No cluster-admin, wildcard, escalation, or broad write permissions were found for this ServiceAccount.',
      'argocd-controller-cluster-admin': 'Apply the Argo CD control-plane baseline first, then remove the old cluster-admin binding. Tenant ServiceAccounts are configured separately.',
      'argocd-sync-impersonation-disabled': 'Set application.sync.impersonation.enabled=true in argocd-cm before using AppProject destination ServiceAccounts.',
      'argocd-no-destination-serviceaccounts': 'No tenant AppProjects use destinationServiceAccounts yet. Configure tenant ServiceAccounts separately after the control-plane baseline.',
      'argocd-version-check': 'Sync impersonation is documented as an alpha feature since Argo CD 2.13. Verify this cluster version before applying the impersonation plan.',
    },
    warningText: {
      highRiskTemplate: 'This is a high-risk template. Review every generated resource before applying.',
      cleanupBindings: 'This plan will remove existing risky RBAC bindings after the new scoped permissions are applied.',
    },
  },
  zh: {
    brand: 'RBAC 权限治理',
    views: { clusters: '集群', tools: '工具', tenants: '租户', templates: '模板', plans: '计划', audit: '审计', 'my-permissions': '我的权限', 'request-permission': '申请权限', 'approval-queue': '权限审批' },
    subtitles: {
      clusters: '导入集群、测试连通性，并执行权限扫描。',
      tools: '查看已发现工具、ServiceAccount 和高风险 Kubernetes RBAC。',
      tenants: '单独创建租户同步 ServiceAccount、AppProject 和命名空间权限绑定，不混在工具控制面权限里。',
      templates: '查看内置权限模板。内置模板随代码版本发布。',
      plans: '确认变更计划后再应用到目标集群。',
      audit: '追踪集群导入、扫描、计划创建、应用和回滚操作。',
      'my-permissions': '查看当前命名空间及工具访问权限，以及历史申请记录。',
      'request-permission': '申请新的命名空间或工具权限。',
      'approval-queue': '批准、驳回或撤销权限申请。',
    },
    refresh: '刷新',
    currentUser: '当前用户',
    user: '用户',
    role: '角色',
    tenantName: '租户名称',
    tenantScope: '租户范围',
    tenantClusters: '集群',
    tenantNamespaces: '命名空间',
    tenantUsers: '用户',
    createTenant: '创建租户',
    createUser: '创建用户',
    tenantGovernance: '租户治理',
    tenantGovernanceHelp: '该页面在 Argo CD 控制面基线完成后使用。租户 ServiceAccount 和 AppProject 必须显式创建，全新 Argo CD 不会默认创建 team-a。',
    tenantTemplate: '租户模板',
    tenantServiceAccount: '租户标识 (SA 名称)',
    tenantServiceAccountHint: '该名称将作为 ServiceAccount 创建在 ArgoCD 命名空间中',
    businessNamespace: '业务命名空间',
    businessNamespaceHint: '该租户将管理的目标命名空间',
    namespacePattern: '命名空间匹配规则',
    namespacePatternHint: '所有匹配此规则的命名空间将自动获得授权',
    tenantLabelKey: '命名空间标签键',
    tenantLabelValue: '命名空间标签值',
    sourceRepo: '允许的 Git 仓库',
    adminGroup: '租户管理员组',
    adminGroupHint: '该组成员将拥有此项目下 Argo CD Application 的同步/查看权限。',
    explicitTenantRequired: '请选择租户模板，并显式填写租户、SA 和命名空间范围。',
    tenantControllerHint: 'Argo CD 命名空间和 controller SA 会从扫描到的 application-controller 工作负载自动获取。',
    tenantSaLocationHint: '租户 SA 将创建在 Argo CD 命名空间（{namespace}）中，便于集中管理。',
    tabLabels: { governance: '租户治理', credential: '凭证生成', scope: '租户管理' },
    governanceState: { secured: '已治理', 'needs-action': '需治理', 'in-progress': '治理中' },
    securedMessage: '权限已收敛到基线',
    clusterOverview: '集群概览',
    importCluster: '导入集群',
    name: '名称',
    kubeconfig: 'Kubeconfig',
    importAndTest: '导入并测试',
    useInCluster: '接入当前集群',
    knownClusters: '已接入集群',
    context: '上下文',
    apiServer: 'API Server',
    lastScan: '最近扫描',
    message: '消息',
    openTools: '打开工具',
    test: '测试',
    scan: '扫描',
    clusterCount: '个集群',
    countSuffix: '个',
    noClusters: '还没有导入集群。',
    cluster: '集群',
    scanSelected: '扫描当前集群',
    detectedTools: '已发现工具',
    namespace: '命名空间',
    kind: '类型',
    serviceAccount: 'ServiceAccount',
    govern: '治理',
    noTools: '还没有发现工具，请先扫描。',
    registerTool: '注册工具识别规则',
    addTool: '新增工具',
    close: '关闭',
    preview: '预览',
    resources: '资源',
    toolRegistry: '工具识别库',
    toolRegistryHelp: '在这里注册新工具的识别规则。扫描时会用这些规则识别任意命名空间里的工作负载。',
    matchText: '匹配文本',
    recommendedTemplates: '推荐模板 ID',
    governanceAction: '治理操作',
    tool: '工具',
    template: '模板',
    targetNamespace: '目标命名空间',
    targetServiceAccount: '目标 ServiceAccount',
    templateParameters: '模板参数',
    previewYaml: '预览 YAML',
    createPlan: '创建计划',
    warning: '警告',
    currentPermissionCheck: '当前权限校验',
    proposedYaml: '拟应用 YAML',
    currentPermissionHelp: '这里显示的是应用计划之前，该 ServiceAccount 当前是否已经具备这些操作权限。',
    proposedYamlHelp: '这里是准备写入集群的新 Role / ClusterRole / RBACDefinition YAML。',
    cleanupOldBindings: '应用后清理旧宽权限绑定',
    cleanupOldBindingsHelp: '先应用新的有限权限模板，再删除本次扫描发现的高风险 RoleBinding 或 ClusterRoleBinding。',
    cleanupBindings: '将删除的旧风险绑定',
    cleanupCandidates: '检测到的旧风险绑定',
    cleanupCandidateHelp: '这些是最近一次扫描发现的已有高风险绑定，不是即将新增的绑定。只有在计划中启用清理时才会删除。',
    noCleanupBindings: '没有选择要清理的高风险绑定。',
    planNote: '当前权限校验和拟应用 YAML 是两件事。前者看现状，后者看变更内容。',
    previewPlaceholder: '预览结果会显示在这里。',
    selectTool: '选择一个工具后预览并应用模板。Argo CD 工具治理这里只处理控制面基线，不会默认创建租户。',
    noTemplatesForTool: '这个组件目前没有预置模板。',
    quickCredential: '为此 SA 生成凭证',
    id: 'ID',
    scope: '范围',
    source: '来源',
    builtIn: '内置',
    custom: '自定义',
    createTemplate: '创建自定义角色模板',
    templateCatalog: '模板目录',
    templateCatalogHelp: '内置模板保存在代码库里，只在预览或创建计划时渲染。',
    builtinTemplates: '内置模板',
    customTemplates: '自定义模板',
    noCustomTemplates: '还没有自定义模板。',
    templatePreviewHelp: '预览模板库中保存的模板源码。内置模板只读；如果新工具需要不同权限，请创建自定义模板。',
    customTemplateHelp: '可以使用 Go 模板占位符，例如 {{ .namespace }} 和 {{ .serviceAccount }}。参数会从 YAML 中自动识别。',
    noParams: '不需要参数。',
    requiredParam: '必填',
    optionalParam: '可选',
    defaultValue: '默认值',
    risk: '风险',
    profile: '档位',
    permissionProfiles: { view: '查看', deploy: '部署', admin: '管理', breakglass: '临时高危' },
    tenantCredential: '租户凭证',
    credentialHelp: '为租户 ServiceAccount 生成短期 token 或 kubeconfig。密钥只返回一次，不会落库。',
    credentialNamespace: '凭证命名空间',
    credentialServiceAccount: '凭证 ServiceAccount',
    credentialExpiration: '有效期秒数',
    credentialFormat: '格式',
    generateCredential: '生成凭证',
    credentialOutput: '凭证输出',
    credentialExpires: '过期时间',
    params: '参数',
    templateId: '模板 ID',
    templateYaml: '模板 YAML 资源',
    created: '创建时间',
    result: '结果',
    validate: '校验',
    apply: '应用',
    rollback: '回滚',
    noPlans: '还没有创建计划。',
    time: '时间',
    action: '动作',
    status: '状态',
    allowed: '允许',
    denied: '拒绝',
    error: '错误',
    confirmApply: '确认要把这个计划应用到目标集群吗？请先检查 YAML。',
    confirmRollback: '确认使用保存的快照回滚这个计划吗？',
    severity: { high: '高危', medium: '中危', low: '低危' },
    statusText: { connected: '已连接', error: '异常', installed: '已安装', missing: '未安装', planned: '已创建', applied: '已应用', failed: '失败', 'rolled-back': '已回滚', success: '成功', unknown: '未知' },
    scopeText: { namespace: '命名空间级', cluster: '集群级', mixed: '混合范围' },
    auditAction: {
      'tenant.create': '创建租户',
      'user.create': '创建用户',
      'cluster.import': '导入集群',
      'cluster.import_in_cluster': '接入集群内环境',
      'cluster.auto_in_cluster': '自动识别集群内环境',
      'cluster.test': '测试集群',
      'cluster.scan': '扫描集群',
      'template.create': '创建模板',
      'tool_profile.create': '创建工具识别规则',
      'tenant.credential.create': '生成租户凭证',
      'plan.create': '创建计划',
      'plan.validate': '校验计划',
      'plan.apply': '应用计划',
      'plan.rollback': '回滚计划',
    },
    findingTitle: {
      'cluster-admin': 'ServiceAccount 绑定了 cluster-admin',
      'wildcard-rbac': '存在通配符 RBAC 权限',
      'read-only-wildcard-rbac': '存在宽泛只读 RBAC 权限',
      'cluster-write': '存在集群范围写权限',
      'secret-read': '可读取 Secret',
      'pod-exec': '具备 Pod exec 权限',
      'privilege-escalation': '存在提权类权限',
      'argocd-tenant-impersonation': 'Argo CD 租户冒充绑定',
      'no-high-risk-rbac': '未发现高风险 RBAC',
      'argocd-controller-cluster-admin': 'Argo CD controller 拥有 cluster-admin',
      'argocd-sync-impersonation-disabled': '未开启同步冒充',
      'argocd-no-destination-serviceaccounts': 'AppProject 未配置 destinationServiceAccounts',
      'argocd-version-check': '需要确认 Argo CD 同步冒充版本支持',
    },
    findingDesc: {
      'cluster-admin': '该权限拥有集群无限制访问能力，应替换为更小范围的模板。',
      'wildcard-rbac': '通配符 verbs、resources 或 apiGroups 会让有效权限难以审计。',
      'read-only-wildcard-rbac': '该绑定可以读取较大范围的资源，但不包含写入或提权动词。',
      'cluster-write': '该绑定授予了超过单命名空间范围的写权限。',
      'secret-read': '该 ServiceAccount 可以读取 Kubernetes Secret，请确认确实需要。',
      'pod-exec': 'Pod exec 可能被用于访问工作负载进程和挂载凭据。',
      'privilege-escalation': '该 ServiceAccount 可以 bind、escalate 或 impersonate 权限。',
      'argocd-tenant-impersonation': '这是租户同步冒充绑定，由租户页管理，不属于工具控制面清理候选。',
      'no-high-risk-rbac': '没有发现 cluster-admin、通配符、提权或宽泛写权限。',
      'argocd-controller-cluster-admin': '应先应用 Argo CD 控制面基线，再移除旧 cluster-admin 绑定。租户 ServiceAccount 需要单独配置。',
      'argocd-sync-impersonation-disabled': '使用 AppProject destinationServiceAccounts 前，需要在 argocd-cm 设置 application.sync.impersonation.enabled=true。',
      'argocd-no-destination-serviceaccounts': '还没有租户 AppProject 使用 destinationServiceAccounts。请在控制面基线完成后单独配置租户 ServiceAccount。',
      'argocd-version-check': '同步冒充从 Argo CD 2.13 起作为 alpha 特性记录，应用前需要确认版本支持。',
    },
    warningText: {
      highRiskTemplate: '这是高风险模板，应用前请逐项检查生成的资源。',
      cleanupBindings: '该计划会在有限权限应用成功后删除已有的高风险 RBAC 绑定。',
    },
  },
} as const

const views = ['clusters', 'tools', 'tenants', 'templates', 'plans', 'audit', 'my-permissions', 'request-permission', 'approval-queue'] as const
type View = (typeof views)[number]

const state = reactive({
  view: 'clusters' as View,
  lang: (localStorage.getItem('lang') === 'en' ? 'en' : 'zh') as Lang,
  clusters: [] as Cluster[],
  tools: [] as Tool[],
  templates: [] as Template[],
  plans: [] as Plan[],
  audit: [] as AuditEvent[],
  permissionRequests: [] as PermissionRequest[],
  me: null as Me | null,
  tenants: [] as Tenant[],
  selectedClusterId: '',
  selectedToolId: '',
  selectedTemplateId: '',
  previewTemplateId: '',
  selectedTenantTemplateId: '',
  showToolModal: false,
  showTemplateModal: false,
  showTenantModal: false,
  showTemplatePreview: false,
  cleanupOldBindings: false,
  renderedYaml: '',
  tenantCredentialOutput: '',
  tenantCredentialExpiresAt: '',
  warnings: [] as string[],
  error: '',
  newTenantName: '',
  newTenantClusters: '*',
  newTenantNamespaces: '*',
  newUserName: '',
  newUserRole: 'tenant-admin',
  newUserTenantIds: 'platform',
  credentialNamespace: '',
  credentialServiceAccount: '',
  credentialExpiration: 28800,
  credentialFormat: 'kubeconfig',
  toolProfile: { type: '', name: '', matchText: '', recommendedTemplateIds: '' },
  customTemplate: { id: '', tool: 'custom', name: '', description: '', scope: 'namespace', riskLevel: 'medium', yaml: '' },
})

const importForm = reactive({ name: '', kubeconfig: '' })
const params = reactive<Record<string, string>>({
  namespace: '',
  controllerServiceAccount: '',
  serviceAccount: '',
  targetNamespace: '',
  targetServiceAccount: '',
  tenant: '',
  namespacePattern: '',
  tenantLabelKey: '',
  tenantLabelValue: '',
  sourceRepo: '',
})
const busy = ref(false)
const navIcons: Record<string, string> = {
  clusters: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M28 6H4a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h24a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2zM4 24V8h24v16H4z"/><path d="M6 10h8v2H6zm0 4h12v2H6zm0 4h8v2H6z"/></svg>',
  tools: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M27 15.54a10.95 10.95 0 0 0 .63-3.54 11 11 0 0 0-11-11 10.95 10.95 0 0 0-3.54.63L11.5 1.5l-2.12 2.12-1.41-1.41L5.85 4.33l-1.41-1.41L2.32 5.04l1.41 1.41L1.61 8.57 3.73 10.7l-1.41 1.41 2.12 2.12 1.41-1.41 2.12 2.12 1.41-1.41A10.95 10.95 0 0 0 11 17a11 11 0 0 0 11 11 10.95 10.95 0 0 0 3.54-.63l2.12 2.12 2.12-2.12-2.12-2.12A10.95 10.95 0 0 0 27 15.54zM16 26a9 9 0 1 1 9-9 9 9 0 0 1-9 9z"/><path d="M18.5 16.5h-5v5h5z"/></svg>',
  tenants: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M16 14a5 5 0 1 0-5-5 5 5 0 0 0 5 5zm0-8a3 3 0 1 1-3 3 3 3 0 0 1 3-3z"/><path d="M26 28H6a2 2 0 0 1-2-2v-4a8 8 0 0 1 8-8h8a8 8 0 0 1 8 8v4a2 2 0 0 1-2 2zM6 26h20v-4a6 6 0 0 0-6-6h-8a6 6 0 0 0-6 6z"/></svg>',
  templates: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M24 4H8a2 2 0 0 0-2 2v20a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2zM8 26V6h16v20H8z"/><path d="M12 10h8v2h-8zm0 4h8v2h-8zm0 4h5v2h-5z"/></svg>',
  plans: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M28 6H4a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h24a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2zM4 24V8h24v16H4z"/><path d="M10 12h12v2H10zm0 4h8v2h-8zm0 4h5v2h-5z"/></svg>',
  audit: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M16 2a14 14 0 1 0 14 14A14 14 0 0 0 16 2zm0 26a12 12 0 1 1 12-12 12 12 0 0 1-12 12z"/><path d="M16 8a1.5 1.5 0 0 0-1.5 1.5v7a1.5 1.5 0 0 0 3 0v-7A1.5 1.5 0 0 0 16 8z"/><circle cx="16" cy="20" r="1.5"/></svg>',
  'my-permissions': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M14 2H4a2 2 0 0 0-2 2v24a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2zm0 26H4V4h10v24z"/><path d="M28 10H20v2h8zm0 4H20v2h8zm0 4H20v2h8zm-6 4h-2v2h2z"/><path d="M8 8h4v2H8zm0 4h4v2H8zm0 4h4v2H8zm0 4h4v2H8z"/></svg>',
  'request-permission': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M16 2a14 14 0 1 0 14 14A14 14 0 0 0 16 2zm0 26a12 12 0 1 1 12-12 12 12 0 0 1-12 12z"/><path d="M17 9h-2v8H9v2h6v6h2v-6h6v-2h-6V9z"/></svg>',
  'approval-queue': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="16" height="16" fill="currentColor"><path d="M28 6H4a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h24a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2zM4 24V8h24v16H4z"/><path d="M10 12h5v2h-5zm0 4h12v2H10zm0 4h8v2h-8z"/><path d="M22 11l-1.5 1.5L23 15l3.5-3.5L25 10l-2 2z"/></svg>',
}
const t = computed(() => messages[state.lang])
const title = computed(() => t.value.views[state.view])
const subtitle = computed(() => t.value.subtitles[state.view])
const visibleTools = computed(() => state.tools)
const currentTool = computed(() => state.tools.find((tool) => tool.id === state.selectedToolId) || visibleTools.value[0] || state.tools[0] || null)
const argoControllerTool = computed(() => state.tools.find((tool) => isArgoControllerTool(tool)) || null)
const candidateTemplates = computed(() => {
  const tool = currentTool.value
  if (!tool) return []
  const recommended = new Set(tool.recommendedTemplateIds)
  if (tool.type === 'argocd') {
    return state.templates.filter((template) => recommended.has(template.id))
  }
  return state.templates.filter((template) => template.tool === tool.type || recommended.has(template.id))
})
const hasToolTemplate = computed(() => candidateTemplates.value.length > 0)
const tenantTemplates = computed(() => state.templates.filter((template) => {
  return template.id === 'argocd-static-tenant'
    || template.id === 'argocd-dynamic-tenant'
    || template.id === 'jenkins-agent-manager'
    || template.id === 'prometheus-namespace-reader'
}))
const selectedTemplate = computed(() => state.templates.find((template) => template.id === state.selectedTemplateId) || null)
const selectedTemplateParams = computed(() => selectedTemplate.value?.params || [])
const canAdmin = computed(() => state.me?.role === 'platform-admin')
const visibleViews = computed(() => views.filter(v => v !== 'approval-queue' || canAdmin.value))
const scopeText = computed(() => t.value.scopeText as Record<string, string>)
const countSuffix = computed(() => t.value.countSuffix || '')
const previewTemplateObject = computed(() => state.templates.find((template) => template.id === state.previewTemplateId) || null)
const builtinTemplates = computed(() => state.templates.filter((template) => template.builtin))
const customTemplates = computed(() => state.templates.filter((template) => !template.builtin))

function switchLang(lang: Lang) {
  state.lang = lang
  localStorage.setItem('lang', lang)
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, { headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(body.error || response.statusText)
  return body as T
}

async function refresh() {
  state.error = ''
  try {
    state.me = await api<Me>('/api/me')
    if (canAdmin.value) state.tenants = await api<Tenant[]>('/api/tenants')
    const [clusters, templates, plans, audit, permissionRequests] = await Promise.all([
      api<Cluster[]>('/api/clusters'),
      api<Template[]>('/api/templates'),
      api<Plan[]>('/api/plans'),
      api<AuditEvent[]>('/api/audit-events'),
      api<PermissionRequest[]>('/api/permission-requests'),
    ])
    state.clusters = clusters
    state.templates = templates
    state.plans = plans
    state.audit = audit
    state.permissionRequests = permissionRequests
    if (!state.selectedClusterId && clusters[0]) state.selectedClusterId = clusters[0].id
    if (state.selectedClusterId) state.tools = await api<Tool[]>(`/api/clusters/${state.selectedClusterId}/tools`)
  } catch (error) {
    setError(error)
  }
}

async function refreshPermissions() {
  try {
    state.permissionRequests = await api<PermissionRequest[]>('/api/permission-requests')
  } catch (error) {
    setError(error)
  }
}

async function submitPermissionRequest(req: { templateId: string; clusterId: string; params: Record<string, string>; reason: string }) {
  await run(async () => {
    await api<PermissionRequest>('/api/permission-requests', {
      method: 'POST',
      body: JSON.stringify(req),
    })
    await refreshPermissions()
    state.view = 'my-permissions'
  })
}

async function approvePermissionRequest(id: string) {
  await run(async () => {
    await api<PermissionRequest>(`/api/permission-requests/${id}/approve`, { method: 'POST' })
    await refreshPermissions()
  })
}

async function rejectPermissionRequest(id: string) {
  await run(async () => {
    await api<PermissionRequest>(`/api/permission-requests/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ rejectReason: 'rejected by admin' }),
    })
    await refreshPermissions()
  })
}

async function revokePermissionRequest(id: string) {
  await run(async () => {
    await api<PermissionRequest>(`/api/permission-requests/${id}/revoke`, { method: 'POST' })
    await refreshPermissions()
  })
}

async function importCluster() {
  await run(async () => {
    const cluster = await api<Cluster>('/api/clusters/import', { method: 'POST', body: JSON.stringify(importForm) })
    state.selectedClusterId = cluster.id
    importForm.kubeconfig = ''
    await refresh()
  })
}

async function importInCluster() {
  await run(async () => {
    const cluster = await api<Cluster>('/api/clusters/in-cluster', { method: 'POST', body: JSON.stringify({ name: 'in-cluster' }) })
    state.selectedClusterId = cluster.id
    await refresh()
  })
}

async function testCluster(id: string) {
  await run(async () => { await api<Cluster>(`/api/clusters/${id}/test`, { method: 'POST' }); await refresh() })
}

async function scanCluster(id: string) {
  await run(async () => {
    state.tools = await api<Tool[]>(`/api/clusters/${id}/scan`, { method: 'POST' })
    await refresh()
  })
}

async function onClusterChange() {
  if (state.selectedClusterId) state.tools = await api<Tool[]>(`/api/clusters/${state.selectedClusterId}/tools`)
}

function governTool(tool: Tool) {
  state.selectedToolId = tool.id
  state.selectedTemplateId = tool.recommendedTemplateIds[0] || candidateTemplates.value[0]?.id || ''
  seedTemplateParams(tool)
  applyTemplateDefaults()
  state.cleanupOldBindings = tool.type !== 'argocd' && hasCleanupCandidate(tool)
  state.renderedYaml = ''
  state.warnings = []
}

function seedTemplateParams(tool: Tool) {
  for (const key of Object.keys(params)) params[key] = ''
  params.namespace = tool.namespace
  params.serviceAccount = tool.type === 'argocd' ? '' : tool.serviceAccount
  if (tool.type === 'argocd') {
    params.controllerServiceAccount = tool.serviceAccount
  }
}

function applyTemplateDefaults() {
  for (const param of selectedTemplateParams.value) {
    if (!params[param.name] && param.default) params[param.name] = param.default
  }
  if (state.selectedTenantTemplateId === 'argocd-dynamic-tenant' && params.serviceAccount) {
    params.namespacePattern = `${params.serviceAccount}-*`
    params.tenant = params.serviceAccount
    params.tenantLabelValue = params.serviceAccount
  }
}

watch(
  () => params.serviceAccount,
  () => {
    if (state.selectedTenantTemplateId === 'argocd-dynamic-tenant') {
      applyTemplateDefaults()
    }
  },
)

async function previewTemplate() {
  await run(async () => {
    if (!state.selectedTemplateId) throw new Error(t.value.noTemplatesForTool)
    const result = await api<{ yaml: string; warnings: string[] }>('/api/templates/render', { method: 'POST', body: JSON.stringify(templateRequest()) })
    state.renderedYaml = result.yaml
    state.warnings = result.warnings || []
  })
}

async function createPlan() {
  await run(async () => {
    if (!state.selectedTemplateId) throw new Error(t.value.noTemplatesForTool)
    const plan = await api<Plan>('/api/plans', { method: 'POST', body: JSON.stringify(templateRequest()) })
    state.renderedYaml = plan.yaml
    state.warnings = plan.warnings || []
    await refresh()
    state.view = 'plans'
  })
}

async function quickCredential(tool: Tool) {
  await run(async () => {
    const result = await api<TenantCredential>('/api/tenants/credentials', {
      method: 'POST',
      body: JSON.stringify({
        clusterId: state.selectedClusterId,
        namespace: tool.namespace,
        serviceAccount: tool.serviceAccount,
        expirationSeconds: 28800,
        format: 'kubeconfig',
      }),
    })
    state.tenantCredentialOutput = result.kubeconfig || result.token || ''
    state.tenantCredentialExpiresAt = result.expiresAt || ''
  })
}

async function previewTenantPlan() {
  await run(async () => {
    const result = await api<{ yaml: string; warnings: string[] }>('/api/templates/render', { method: 'POST', body: JSON.stringify(tenantTemplateRequest()) })
    state.renderedYaml = result.yaml
    state.warnings = result.warnings || []
  })
}

async function createTenantPlan() {
  await run(async () => {
    const plan = await api<Plan>('/api/plans', { method: 'POST', body: JSON.stringify(tenantTemplateRequest()) })
    state.renderedYaml = plan.yaml
    state.warnings = plan.warnings || []
    await refresh()
    state.view = 'plans'
  })
}

async function createTenantCredential() {
  await run(async () => {
    const result = await api<TenantCredential>('/api/tenants/credentials', {
      method: 'POST',
      body: JSON.stringify({
        clusterId: state.selectedClusterId,
        namespace: state.credentialNamespace,
        serviceAccount: state.credentialServiceAccount,
        expirationSeconds: Number(state.credentialExpiration) || 28800,
        format: state.credentialFormat,
      }),
    })
    state.tenantCredentialOutput = result.kubeconfig || result.token || ''
    state.tenantCredentialExpiresAt = result.expiresAt || ''
    await refresh()
  })
}

async function validatePlan(plan: Plan) {
  await run(async () => { await api<Plan>(`/api/plans/${plan.id}/validate`, { method: 'POST' }); await refresh() })
}

async function applyPlan(plan: Plan) {
  if (!window.confirm(t.value.confirmApply)) return
  await run(async () => {
    await api<Plan>(`/api/plans/${plan.id}/apply`, { method: 'POST' })
    if (state.selectedClusterId) {
      state.tools = await api<Tool[]>(`/api/clusters/${state.selectedClusterId}/scan`, { method: 'POST' })
    }
    await refresh()
  })
}

async function rollbackPlan(plan: Plan) {
  if (!window.confirm(t.value.confirmRollback)) return
  await run(async () => { await api<Plan>(`/api/plans/${plan.id}/rollback`, { method: 'POST' }); await refresh() })
}

async function createTenant() {
  await run(async () => {
    await api<Tenant>('/api/tenants', {
      method: 'POST',
      body: JSON.stringify({
        name: state.newTenantName,
        clusterIds: state.newTenantClusters.split(',').map((x) => x.trim()).filter(Boolean),
        namespaces: state.newTenantNamespaces.split(',').map((x) => x.trim()).filter(Boolean),
      }),
    })
    state.newTenantName = ''
    state.newTenantClusters = '*'
    state.newTenantNamespaces = '*'
    await refresh()
  })
}

async function createUser() {
  await run(async () => {
    await api<User>('/api/users', {
      method: 'POST',
      body: JSON.stringify({
        name: state.newUserName,
        role: state.newUserRole,
        tenantIds: state.newUserTenantIds.split(',').map((x) => x.trim()).filter(Boolean),
      }),
    })
    state.newUserName = ''
    state.newUserRole = 'tenant-admin'
    state.newUserTenantIds = 'platform'
    await refresh()
  })
}

async function createToolProfile() {
  await run(async () => {
    await api<ToolProfile>('/api/tool-profiles', {
      method: 'POST',
      body: JSON.stringify({
        type: state.toolProfile.type,
        name: state.toolProfile.name,
        matchText: state.toolProfile.matchText,
        recommendedTemplateIds: state.toolProfile.recommendedTemplateIds.split(',').map((x) => x.trim()).filter(Boolean),
      }),
    })
    state.toolProfile = { type: '', name: '', matchText: '', recommendedTemplateIds: '' }
    state.showToolModal = false
    await refresh()
  })
}

async function createCustomTemplate() {
  await run(async () => {
    const inferredParams = inferTemplateParams(state.customTemplate.yaml)
    await api<Template>('/api/templates', {
      method: 'POST',
      body: JSON.stringify({
        id: state.customTemplate.id,
        tool: state.customTemplate.tool,
        name: state.customTemplate.name,
        description: state.customTemplate.description,
        scope: state.customTemplate.scope,
        riskLevel: state.customTemplate.riskLevel,
        builtin: false,
        params: inferredParams,
        resources: [{ kind: 'Custom', template: state.customTemplate.yaml }],
      }),
    })
    state.customTemplate = { id: '', tool: 'custom', name: '', description: '', scope: 'namespace', riskLevel: 'medium', yaml: '' }
    state.showTemplateModal = false
    await refresh()
  })
}

function chooseTenantTemplate(templateId: string) {
  state.selectedTenantTemplateId = templateId
  const template = state.templates.find((item) => item.id === templateId)
  if (!template) return
  for (const key of Object.keys(params)) params[key] = ''
  if (templateId.startsWith('argocd-')) {
    params.namespace = argoControllerTool.value?.namespace || ''
    params.controllerServiceAccount = argoControllerTool.value?.serviceAccount || ''
  }
  if (templateId === 'jenkins-agent-manager') {
    params.serviceAccount = 'jenkins'
  } else if (templateId === 'prometheus-namespace-reader') {
    params.serviceAccount = 'prometheus'
  }
  params.targetNamespace = ''
  params.tenant = ''
  params.namespacePattern = ''
  params.tenantLabelKey = 'tenant'
  params.tenantLabelValue = ''
  params.sourceRepo = '*'
  state.selectedTemplateId = template.id
  applyTemplateDefaults()
}

function inferTemplateParams(yaml: string) {
  const names = new Set<string>()
  const regex = /\{\{\s*(?:dns\s+)?\.([A-Za-z][A-Za-z0-9_]*)\s*\}\}/g
  let match: RegExpExecArray | null
  while ((match = regex.exec(yaml)) !== null) names.add(match[1])
  if (!names.size) {
    names.add('namespace')
    names.add('serviceAccount')
  }
  return Array.from(names).sort().map((name) => ({ name, label: paramDisplayName(name), required: true }))
}

function paramDisplayName(name: string) {
  return localizedParamLabel({ name, label: name })
}

function openTemplatePreview(template: Template) {
  state.previewTemplateId = template.id
  state.showTemplatePreview = true
}

function onTemplateChange(name?: string, value?: string) {
  if (name === 'templateId' && value) {
    state.selectedTemplateId = value
  } else if (name && value !== undefined) {
    params[name] = value
  }
  applyTemplateDefaults()
  state.renderedYaml = ''
  state.warnings = []
}

function templateRequest() {
  applyTemplateDefaults()
  return { clusterId: state.selectedClusterId, toolId: state.selectedToolId, templateId: state.selectedTemplateId, params: { ...params }, cleanup: state.cleanupOldBindings }
}

function tenantTemplateRequest() {
  applyTemplateDefaults()
  return {
    clusterId: state.selectedClusterId,
    toolId: '',
    templateId: state.selectedTenantTemplateId || state.selectedTemplateId,
    params: { ...params, controllerServiceAccount: params.controllerServiceAccount || argoControllerTool.value?.serviceAccount || '' },
    cleanup: false,
  }
}

function isArgoControllerTool(tool: Tool) {
  const component = (tool.labels?.['app.kubernetes.io/component'] || '').toLowerCase()
  const labelName = (tool.labels?.['app.kubernetes.io/name'] || '').toLowerCase()
  const name = tool.name.toLowerCase()
  return tool.type === 'argocd' && (component === 'application-controller' || name.includes('application-controller') || labelName.includes('application-controller'))
}

async function run(fn: () => Promise<unknown>) {
  busy.value = true
  state.error = ''
  try { await fn() } catch (error) { setError(error) } finally { busy.value = false }
}

function setError(error: unknown) {
  state.error = error instanceof Error ? error.message : String(error)
}

function maxSeverity(findings: Finding[]) {
  if (findings.some((finding) => finding.severity === 'high')) return 'high'
  if (findings.some((finding) => finding.severity === 'medium')) return 'medium'
  return 'low'
}

function statusLabel(value?: string) {
  if (!value) return '-'
  return (t.value.statusText as Record<string, string>)[value] || value
}

function auditActionLabel(value: string) {
  return (t.value.auditAction as Record<string, string>)[value] || value
}

function messageLabel(value?: string) {
  if (!value) return '-'
  if (state.lang === 'en') return value
  const zhMessages: Record<string, string> = {
    connected: '连接正常',
    'auto-detected in-cluster connection': '已自动识别集群内连接',
    'plan created': '计划已创建',
    'plan validated': '计划已校验',
    'applied successfully': '应用成功',
    'rolled back successfully': '回滚成功',
  }
  if (zhMessages[value]) return zhMessages[value]
  const discovered = value.match(/^discovered (\d+) tool instances$/)
  if (discovered) return `发现 ${discovered[1]} 个工具实例`
  return value
}

function warningLabel(value: string) {
  if (state.lang === 'en') return value
  if (value.includes('high-risk template')) return t.value.warningText.highRiskTemplate
  if (value.includes('risky RBAC binding')) return t.value.warningText.cleanupBindings
  return value
}

function severityLabel(value: Finding['severity']) {
  return t.value.severity[value]
}

function findingTitle(finding: Finding) {
  return (t.value.findingTitle as Record<string, string>)[finding.ruleId] || finding.title
}

function findingDescription(finding: Finding) {
  return (t.value.findingDesc as Record<string, string>)[finding.ruleId] || finding.description
}

function hasCleanupCandidate(tool: Tool) {
  return tool.findings.some((finding) => cleanupEligibleRule(finding.ruleId) && bindingResource(finding.resource))
}

function cleanupEligibleRule(ruleId: string) {
  return ['cluster-admin', 'wildcard-rbac', 'cluster-write', 'pod-exec', 'privilege-escalation', 'argocd-controller-cluster-admin'].includes(ruleId)
}

function bindingResource(resource: string) {
  return /^ClusterRoleBinding\/[^/]+$/.test(resource) || /^[^/]+\/RoleBinding\/[^/]+$/.test(resource)
}

function cleanupCandidates(tool: Tool | null) {
  if (!tool) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const finding of tool.findings) {
    if (!cleanupEligibleRule(finding.ruleId) || !bindingResource(finding.resource) || seen.has(finding.resource)) continue
    seen.add(finding.resource)
    out.push(finding.resource)
  }
  return out
}

function localizedTemplateName(template: Template) {
  return localizedTemplateMeta(template).name
}

function localizedTemplateDescription(template: Template) {
  return localizedTemplateMeta(template).description || template.description
}

function localizedTemplateMeta(template: Template) {
  const meta = templateLocaleMeta[state.lang][template.id]
  return meta || { name: template.name, description: template.description }
}

function permissionProfile(template: Template) {
  if (template.riskLevel === 'high') return template.id.includes('breakglass') ? 'breakglass' : 'admin'
  if (template.scope === 'cluster') return template.riskLevel === 'low' ? 'view' : 'admin'
  if (template.id.includes('reader') || template.id.includes('read-only')) return 'view'
  if (template.id.includes('edit') || template.id.includes('sync')) return 'deploy'
  return template.riskLevel === 'low' ? 'view' : 'deploy'
}

function permissionProfileLabel(template: Template) {
  const profile = permissionProfile(template)
  return (t.value.permissionProfiles as Record<string, string>)[profile] || profile
}

function localizedParamLabel(param: { name: string; label: string }) {
  const labels: Record<Lang, Record<string, string>> = {
    en: {},
    zh: {
      namespace: 'Argo CD 命名空间',
      controllerServiceAccount: 'Argo CD Controller ServiceAccount',
      serviceAccount: '租户标识 (SA 名称)',
      targetNamespace: '业务命名空间',
      targetServiceAccount: '目标 ServiceAccount',
      tenant: '租户名称',
      namespacePattern: '命名空间匹配规则',
      tenantLabelKey: '命名空间标签键',
      tenantLabelValue: '命名空间标签值',
      sourceRepo: '允许的 Git 仓库',
      adminGroup: '租户管理员组',
    },
  }
  return labels[state.lang][param.name] || param.label || param.name
}

const templateLocaleMeta: Record<Lang, Record<string, { name: string; description: string }>> = {
  en: {
    'namespace-editor': {
      name: 'Namespace editor',
      description: 'Grants full edit access to common resources in a specific namespace.',
    },
    'argocd-control-plane': {
      name: 'Argo CD control-plane permissions',
      description: 'Grants the Argo CD controller read-only access across the cluster.',
    },
    'argocd-static-tenant': {
      name: 'Argo CD static tenant permissions',
      description: 'Single-namespace tenant. Creates SA in Argo CD namespace, AppProject, and namespace-scoped RBAC.',
    },
    'argocd-dynamic-tenant': {
      name: 'Argo CD dynamic tenant permissions',
      description: 'Multi-namespace tenant. Uses label selector to grant access to dynamic namespaces.',
    },
    'argocd-control-plane-read-impersonate': {
      name: 'Argo CD control-plane read and impersonate',
      description: 'Grants Argo CD read/watch permissions and impersonation of a specific tenant ServiceAccount. AppProject changes are still required.',
    },
    'jenkins-agent-manager': {
      name: 'Jenkins agent manager',
      description: 'Lets Jenkins manage build agent Pods in one namespace.',
    },
    'jenkins-namespace-edit': {
      name: 'Jenkins namespace edit',
      description: 'Lets Jenkins deploy common workload resources in one namespace.',
    },
    'prometheus-cluster-reader': {
      name: 'Prometheus cluster reader',
      description: 'Read-only cluster discovery for Prometheus without write permissions.',
    },
    'prometheus-namespace-reader': {
      name: 'Prometheus namespace reader',
      description: 'Namespace-scoped discovery for Prometheus.',
    },
    'loki-namespace-reader': {
      name: 'Loki namespace reader',
      description: 'Minimal namespace metadata read permissions for Loki components.',
    },
    'promtail-cluster-metadata-reader': {
      name: 'Log collector metadata reader',
      description: 'Read-only metadata discovery for Promtail, Grafana Agent, or Alloy.',
    },
  },
  zh: {
    'namespace-editor': {
      name: 'Namespace 编辑权限',
      description: '授予对指定命名空间中常见资源的完整编辑权限。',
    },
    'argocd-control-plane': {
      name: 'Argo CD 控制面权限',
      description: '授予 Argo CD 控制器集群范围的只读访问权限。',
    },
    'argocd-static-tenant': {
      name: 'Argo CD 静态租户权限',
      description: '单命名空间租户。在 Argo CD 命名空间创建 SA、AppProject 及 RBAC。',
    },
    'argocd-dynamic-tenant': {
      name: 'Argo CD 动态租户权限',
      description: '多命名空间租户。通过标签选择器授权访问动态命名空间。',
    },
    'argocd-control-plane-read-impersonate': {
      name: 'Argo CD 控制面只读与冒充',
      description: '授予 Argo CD 只读/观察权限，以及对指定租户 ServiceAccount 的冒充能力。仍需配合 AppProject 修改。',
    },
    'jenkins-agent-manager': {
      name: 'Jenkins Agent 管理器',
      description: '允许 Jenkins 在单个命名空间内管理构建 Agent Pod。',
    },
    'jenkins-namespace-edit': {
      name: 'Jenkins 命名空间编辑',
      description: '让 Jenkins 在单个命名空间中部署常见工作负载资源。',
    },
    'prometheus-cluster-reader': {
      name: 'Prometheus 集群读者',
      description: '仅提供只读的集群发现权限，不包含写权限。',
    },
    'prometheus-namespace-reader': {
      name: 'Prometheus 命名空间读者',
      description: '为 Prometheus 提供命名空间范围的发现权限。',
    },
    'loki-namespace-reader': {
      name: 'Loki 命名空间读者',
      description: '为 Loki 组件提供最小化的命名空间元数据读取权限。',
    },
    'promtail-cluster-metadata-reader': {
      name: '日志采集元数据读者',
      description: '为 Promtail、Grafana Agent 或 Alloy 提供只读元数据发现能力。',
    },
  },
}

function formatTime(value?: string) {
  if (!value || value.startsWith('0001-')) return '-'
  return new Date(value).toLocaleString()
}

onMounted(refresh)
</script>

<template>
  <div id="app-shell">
    <aside>
      <div class="brand">{{ t.brand }}</div>
      <nav>
        <button v-for="view in visibleViews" :key="view" class="nav" :class="{ active: state.view === view }" @click="state.view = view">
          <span class="nav-icon" v-html="navIcons[view]"></span>
          {{ t.views[view] }}
        </button>
      </nav>
    </aside>

    <main>
      <header>
        <div>
          <h1>{{ title }}</h1>
          <p>{{ subtitle }}</p>
        </div>
        <div class="row">
          <button :class="{ primary: state.lang === 'zh' }" @click="switchLang('zh')">中文</button>
          <button :class="{ primary: state.lang === 'en' }" @click="switchLang('en')">EN</button>
          <button :disabled="busy" @click="refresh">{{ t.refresh }}</button>
        </div>
      </header>

      <section v-if="state.error" class="finding high error-box"><strong>{{ t.error }}</strong><div>{{ state.error }}</div></section>

      <ClusterPanel
        v-if="state.view === 'clusters'"
        :clusters="state.clusters"
        :can-admin="canAdmin"
        :me="state.me"
        :import-form="importForm"
        :t="t"
        :lang="state.lang"
        @import-cluster="importCluster"
        @import-in-cluster="importInCluster"
        @test-cluster="testCluster"
        @scan-cluster="scanCluster"
        @open-tools="(cluster) => { state.selectedClusterId = cluster.id; state.view = 'tools'; onClusterChange() }"
        @update:import-form="(v) => Object.assign(importForm, v)"
      />

      <section v-else-if="state.view === 'tools'" class="stack">
        <div class="toolbar">
          <label class="cluster-picker">{{ t.cluster }}
            <select v-model="state.selectedClusterId" @change="onClusterChange">
              <option v-for="cluster in state.clusters" :key="cluster.id" :value="cluster.id">{{ cluster.name }}</option>
            </select>
          </label>
          <div class="toolbar-actions">
            <button v-if="canAdmin" @click="state.showToolModal = true">{{ t.addTool }}</button>
            <button class="primary" :disabled="!state.selectedClusterId || busy" @click="scanCluster(state.selectedClusterId)">{{ t.scanSelected }}</button>
          </div>
        </div>

        <div class="tool-layout">
          <ToolList
            :tools="visibleTools"
            :current-tool="currentTool"
            :count-suffix="countSuffix"
            :t="t"
            @govern-tool="governTool"
          />
          <GovernancePanel
            :current-tool="currentTool"
            :candidate-templates="candidateTemplates"
            :has-tool-template="hasToolTemplate"
            :selected-template-id="state.selectedTemplateId"
            :selected-template-params="selectedTemplateParams"
            :cleanup-old-bindings="state.cleanupOldBindings"
            :rendered-yaml="state.renderedYaml"
            :warnings="state.warnings"
            :t="{ ...t, localizedTemplateName, localizedParamLabel, permissionProfileLabel, warningLabel, lang: state.lang }"
            :params="params"
            @template-change="onTemplateChange"
            @preview="previewTemplate"
            @create-plan="createPlan"
            @quick-credential="quickCredential"
            @update:cleanup-old-bindings="(v) => { state.cleanupOldBindings = v }"
          />
        </div>
      </section>

      <section v-else-if="state.view === 'tenants'">
        <TenantPanel
          :clusters="state.clusters"
          :selected-cluster-id="state.selectedClusterId"
          :tenant-templates="tenantTemplates"
          :selected-tenant-template-id="state.selectedTenantTemplateId"
          :params="params"
          :rendered-yaml="state.renderedYaml"
          :warnings="state.warnings"
          :credential-namespace="state.credentialNamespace"
          :credential-service-account="state.credentialServiceAccount"
          :credential-expiration="state.credentialExpiration"
          :credential-format="state.credentialFormat"
          :tenant-credential-output="state.tenantCredentialOutput"
          :tenant-credential-expires-at="state.tenantCredentialExpiresAt"
          :tenants="state.tenants"
          :can-admin="canAdmin"
          :lang="state.lang"
          :t="{ ...t, localizedTemplateName, warningLabel }"
          @cluster-change="onClusterChange"
          @tenant-template-change="chooseTenantTemplate"
          @preview-tenant-plan="previewTenantPlan"
          @create-tenant-plan="createTenantPlan"
          @create-tenant-credential="createTenantCredential"
          @open-tenant-modal="state.showTenantModal = true"
          @update:params="(p: Record<string, string>) => { Object.assign(params, p) }"
          @update:credential-namespace="(v: string) => { state.credentialNamespace = v }"
          @update:credential-service-account="(v: string) => { state.credentialServiceAccount = v }"
          @update:credential-expiration="(v: number) => { state.credentialExpiration = v }"
          @update:credential-format="(v: string) => { state.credentialFormat = v }"
        />
      </section>

      <section v-else-if="state.view === 'templates'">
        <TemplatePanel
          :builtin-templates="builtinTemplates"
          :custom-templates="customTemplates"
          :can-admin="canAdmin"
          :scope-text="scopeText"
          :t="{ ...t, localizedTemplateName, localizedTemplateDescription, permissionProfileLabel }"
          @open-template-modal="state.showTemplateModal = true"
          @open-template-preview="openTemplatePreview"
        />
      </section>

      <section v-else-if="state.view === 'plans'" class="grid">
        <article v-for="plan in state.plans" :key="plan.id" class="card">
          <div class="row"><div class="card-title">{{ plan.templateId }}</div><span class="badge" :class="plan.status === 'applied' ? 'success' : plan.status === 'failed' ? 'high' : 'medium'">{{ statusLabel(plan.status) }}</span></div>
          <div class="kv"><span>{{ t.cluster }}</span><span class="mono">{{ plan.clusterId }}</span><span>{{ t.tool }}</span><span class="mono">{{ plan.toolId || '-' }}</span><span>{{ t.created }}</span><span>{{ formatTime(plan.createdAt) }}</span><span>{{ t.result }}</span><span>{{ messageLabel(plan.result) }}</span></div>
          <p class="notice">{{ t.planNote }}</p>
          <div v-for="warning in plan.warnings" :key="warning" class="finding medium"><strong>{{ t.warning }}</strong><div class="small">{{ warningLabel(warning) }}</div></div>
          <div v-if="plan.cleanup?.length" class="cleanup-list">
            <div class="subsection-title">{{ t.cleanupBindings }}</div>
            <div class="pill-row">
              <span v-for="item in plan.cleanup" :key="item.kind + item.namespace + item.name" class="badge high mono">{{ item.namespace ? `${item.namespace}/${item.kind}/${item.name}` : `${item.kind}/${item.name}` }}</span>
            </div>
          </div>
          <div class="plan-grid">
            <div class="subsection">
              <div class="subsection-title">{{ t.currentPermissionCheck }}</div>
              <p>{{ t.currentPermissionHelp }}</p>
              <div v-if="plan.validation?.length" class="stack">
                <div v-for="check in plan.validation" :key="check.namespace + check.resource + check.verb" class="finding" :class="check.allowed ? 'low' : 'high'">
                  <div class="row"><strong>{{ check.verb }} {{ check.resource }}</strong><span class="badge" :class="check.allowed ? 'success' : 'high'">{{ check.allowed ? t.allowed : t.denied }}</span></div>
                  <div class="small muted">{{ check.serviceAccount }} @ {{ check.namespace || '-' }}</div><div class="small">{{ check.reason }}</div>
                </div>
              </div>
              <div v-else class="empty">{{ t.currentPermissionCheck }}</div>
            </div>
            <div class="subsection">
              <div class="subsection-title">{{ t.proposedYaml }}</div>
              <p>{{ t.proposedYamlHelp }}</p>
              <pre>{{ plan.yaml }}</pre>
            </div>
          </div>
          <div class="row"><button @click="validatePlan(plan)">{{ t.validate }}</button><button class="primary" :disabled="plan.status === 'applied' || busy" @click="applyPlan(plan)">{{ t.apply }}</button><button class="danger" :disabled="busy || !plan.rollback?.length" @click="rollbackPlan(plan)">{{ t.rollback }}</button></div>
        </article>
        <div v-if="!state.plans.length" class="empty">{{ t.noPlans }}</div>
      </section>

      <MyPermissions
        v-else-if="state.view === 'my-permissions'"
        :lang="state.lang"
        :current-user="state.me || {id:'',name:'',role:''}"
        :templates="state.templates"
        :permission-requests="state.permissionRequests"
        @navigate-request="state.view = 'request-permission'"
      />

      <RequestForm
        v-else-if="state.view === 'request-permission'"
        :lang="state.lang"
        :clusters="state.clusters"
        :templates="state.templates"
        @submit-request="submitPermissionRequest"
      />

      <PermissionAdmin
        v-else-if="state.view === 'approval-queue'"
        :lang="state.lang"
        :is-admin="canAdmin"
        :permission-requests="state.permissionRequests"
        @refresh="refreshPermissions"
        @approve-request="approvePermissionRequest"
        @reject-request="(id) => rejectPermissionRequest(id)"
        @revoke-request="revokePermissionRequest"
      />

      <section v-else-if="state.view === 'audit'" class="panel">
        <table class="table">
          <thead><tr><th>{{ t.time }}</th><th>{{ t.action }}</th><th>{{ t.status }}</th><th>{{ t.cluster }}</th><th>{{ t.message }}</th></tr></thead>
          <tbody>
            <tr v-for="event in state.audit" :key="event.id">
              <td>{{ formatTime(event.createdAt) }}</td><td>{{ auditActionLabel(event.action) }}</td><td><span class="badge" :class="event.status === 'success' ? 'success' : event.status === 'failed' || event.status === 'error' ? 'high' : 'medium'">{{ statusLabel(event.status) }}</span></td><td class="mono">{{ event.clusterId || '-' }}</td><td>{{ messageLabel(event.message) }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <div v-if="state.showToolModal" class="modal-backdrop" @click.self="state.showToolModal = false">
        <section class="modal">
          <div class="section-head"><div><h2>{{ t.registerTool }}</h2><p>{{ t.toolRegistryHelp }}</p></div><button @click="state.showToolModal = false">{{ t.close }}</button></div>
          <div class="stack">
            <label>{{ t.tool }} <input v-model="state.toolProfile.type" placeholder="trivy" /></label>
            <label>{{ t.name }} <input v-model="state.toolProfile.name" placeholder="Trivy" /></label>
            <label>{{ t.matchText }} <input v-model="state.toolProfile.matchText" placeholder="trivy,aqua" /></label>
            <label>{{ t.recommendedTemplates }} <input v-model="state.toolProfile.recommendedTemplateIds" placeholder="prometheus-namespace-reader" /></label>
            <div class="row end"><button class="primary" @click="createToolProfile">{{ t.registerTool }}</button></div>
          </div>
        </section>
      </div>

      <div v-if="state.showTenantModal" class="modal-backdrop" @click.self="state.showTenantModal = false">
        <section class="modal">
          <div class="section-head"><div><h2>{{ t.createTenant }}</h2><p>{{ state.lang === 'zh' ? '这里只创建控制台租户范围，不会自动创建 Argo CD tenant SA。' : 'This creates console tenant scope only. It does not automatically create an Argo CD tenant ServiceAccount.' }}</p></div><button @click="state.showTenantModal = false">{{ t.close }}</button></div>
          <div class="stack">
            <label>{{ t.tenantName }} <input v-model="state.newTenantName" placeholder="team-a" /></label>
            <label>{{ t.tenantClusters }} <input v-model="state.newTenantClusters" placeholder="kind-rbac-manager-test,*" /></label>
            <label>{{ t.tenantNamespaces }} <input v-model="state.newTenantNamespaces" placeholder="team-a,team-b,*" /></label>
            <div class="row end"><button class="primary" :disabled="busy || !state.newTenantName" @click="createTenant(); state.showTenantModal = false">{{ t.createTenant }}</button></div>
          </div>
        </section>
      </div>

      <div v-if="state.showTemplateModal" class="modal-backdrop" @click.self="state.showTemplateModal = false">
        <section class="modal wide">
          <div class="section-head"><div><h2>{{ t.createTemplate }}</h2><p>{{ t.customTemplateHelp }}</p></div><button @click="state.showTemplateModal = false">{{ t.close }}</button></div>
          <div class="template-form">
            <div class="stack">
              <label>{{ t.templateId }} <input v-model="state.customTemplate.id" placeholder="custom.namespace-reader" /></label>
              <label>{{ t.tool }} <input v-model="state.customTemplate.tool" placeholder="custom" /></label>
              <label>{{ t.name }} <input v-model="state.customTemplate.name" /></label>
              <label>{{ t.message }} <input v-model="state.customTemplate.description" /></label>
              <div class="grid two">
                <label><span class="field-label">{{ t.scope }}</span>
                  <select v-model="state.customTemplate.scope">
                    <option value="namespace">{{ scopeText.namespace }}</option>
                    <option value="cluster">{{ scopeText.cluster }}</option>
                    <option value="mixed">{{ scopeText.mixed }}</option>
                  </select>
                </label>
                <label><span class="field-label">{{ t.risk }}</span>
                  <select v-model="state.customTemplate.riskLevel">
                    <option value="low">{{ severityLabel('low') }}</option>
                    <option value="medium">{{ severityLabel('medium') }}</option>
                    <option value="high">{{ severityLabel('high') }}</option>
                  </select>
                </label>
              </div>
              <button class="primary" @click="createCustomTemplate">{{ t.createTemplate }}</button>
            </div>
            <label>{{ t.templateYaml }} <textarea v-model="state.customTemplate.yaml" placeholder="apiVersion: rbac.authorization.k8s.io/v1&#10;kind: ClusterRole&#10;metadata:&#10;  name: custom-role" /></label>
          </div>
        </section>
      </div>

      <div v-if="state.showTemplatePreview && previewTemplateObject" class="modal-backdrop" @click.self="state.showTemplatePreview = false">
        <section class="modal wide">
          <div class="section-head">
            <div>
              <h2>{{ localizedTemplateName(previewTemplateObject) }}</h2>
              <p>{{ localizedTemplateDescription(previewTemplateObject) }}</p>
              <p class="small">{{ t.templatePreviewHelp }}</p>
            </div>
            <button @click="state.showTemplatePreview = false">{{ t.close }}</button>
          </div>
          <div class="template-meta">
            <span class="badge">{{ previewTemplateObject.tool }}</span>
            <span class="badge">{{ permissionProfileLabel(previewTemplateObject) }}</span>
            <span class="badge">{{ scopeText[previewTemplateObject.scope] || previewTemplateObject.scope }}</span>
          </div>
          <div class="cleanup-list" style="margin-top: 12px">
            <div class="subsection-title">{{ t.params }}</div>
            <div v-if="previewTemplateObject.params.length" class="param-list">
              <div v-for="param in previewTemplateObject.params" :key="param.name" class="param-row">
                <span><strong>{{ localizedParamLabel(param) }}</strong><small class="mono">{{ param.name }}</small></span>
                <span class="badge" :class="param.required ? 'medium' : 'low'">{{ param.required ? t.requiredParam : t.optionalParam }}</span>
                <span class="small muted">{{ param.default ? `${t.defaultValue}: ${param.default}` : '-' }}</span>
              </div>
            </div>
            <div v-else class="small muted">{{ t.noParams }}</div>
          </div>
          <div class="stack" style="margin-top: 12px">
            <div v-for="resource in previewTemplateObject.resources" :key="resource.kind" class="subsection">
              <div class="subsection-title">{{ resource.kind }}</div>
              <pre>{{ resource.template }}</pre>
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>
