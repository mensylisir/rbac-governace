<script setup lang="ts">
import { computed } from 'vue'

export type Template = { id: string; tool: string; name: string; riskLevel: string; params?: any[]; resources?: Array<{ kind: string; template: string }> }

export type PermissionRequest = {
  id: string
  requesterId: string
  templateId: string
  clusterId: string
  params: Record<string, string>
  reason: string
  riskLevel: string
  status: 'pending' | 'auto-approved' | 'approved' | 'rejected' | 'applied' | 'revoked' | 'failed'
  approverId?: string
  planId?: string
  rejectReason?: string
  createdAt: string
  resolvedAt?: string
}

type MyPermissionsView = {
  user: { id: string; name: string; groups: string[] }
  namespaces: Array<{ name: string; access: string; resources: string[]; source: any }>
  tools: Array<{ tool: string; name: string; namespace?: string; permissions: string[]; source: any }>
  requests: PermissionRequest[]
}

const props = defineProps<{
  lang: 'zh' | 'en'
  currentUser: { id: string; name: string; role: string }
  templates: Array<{ id: string; tool: string; name: string; riskLevel: string; params: Array<{ name: string; label: string; required: boolean; default?: string }>; resources?: Array<{ kind: string; template: string }> }>
  permissionRequests: PermissionRequest[]
}>()

const emit = defineEmits<{
  navigateRequest: []
}>()

function extractResourcesFromTemplate(tmpl: Template) {
  const resources: string[] = []
  for (const res of tmpl.resources || []) {
    if (res.kind === 'ClusterRole' || res.kind === 'Role') {
      if (res.template.includes('deployments')) resources.push('deployments')
      if (res.template.includes('services')) resources.push('services')
      if (res.template.includes('configmaps')) resources.push('configmaps')
      if (res.template.includes('secrets')) resources.push('secrets')
      if (res.template.includes('jobs') || res.template.includes('cronjobs')) resources.push('jobs')
      if (res.template.includes('ingresses')) resources.push('ingresses')
      if (res.template.includes('pods')) resources.push('pods')
    }
  }
  return resources
}

function extractToolPermissions(tmpl: Template, params: Record<string, string>) {
  const perms: string[] = []
  switch (tmpl.id) {
    case 'argocd-static-tenant':
    case 'argocd-dynamic-tenant':
      perms.push('applications:*', 'projects:get')
      break
    case 'argocd-control-plane':
      perms.push('cluster:argocd-application-controller-read')
      break
    case 'prometheus-cluster-reader':
      perms.push('metrics:read', 'discovery:cluster-wide')
      break
    case 'prometheus-namespace-reader':
      perms.push('metrics:read', 'discovery:namespace-scoped')
      break
    case 'jenkins-agent-manager':
      perms.push('ci:manage-agents')
      break
    case 'jenkins-namespace-edit':
      perms.push('ci:deploy')
      break
  }
  return perms
}

const requests = computed(() => props.permissionRequests)

const myPermissions = computed<MyPermissionsView>(() => {
  const namespaces: MyPermissionsView['namespaces'] = []
  const tools: MyPermissionsView['tools'] = []
  for (const pr of requests.value) {
    if (pr.status !== 'applied') continue
    const tmpl = props.templates.find(t => t.id === pr.templateId)
    if (!tmpl) continue
    const source = { templateId: pr.templateId, requestedAt: pr.createdAt, approverId: pr.approverId || '' }
    const ns = pr.params['namespace'] || pr.params['targetNamespace'] || ''
    if (ns) {
      const res = extractResourcesFromTemplate(tmpl)
      namespaces.push({ name: ns, access: tmpl.riskLevel, resources: res, source })
    }
    const perms = extractToolPermissions(tmpl, pr.params)
    if (perms.length) {
      tools.push({ tool: tmpl.tool, name: pr.params['serviceAccount'] || '', namespace: ns, permissions: perms, source })
    }
  }
  return { user: { id: props.currentUser.id, name: props.currentUser.name, groups: [] }, namespaces, tools, requests: requests.value }
})

const t = computed(() => {
  const msgs: Record<string, Record<string, string>> = {
    myPermissionsTitle: { zh: '我的权限', en: 'My Permissions' },
    namespace: { zh: '命名空间', en: 'Namespace' },
    access: { zh: '权限级别', en: 'Access' },
    resources: { zh: '可操作资源', en: 'Resources' },
    tools: { zh: '工具权限', en: 'Tool Permissions' },
    permissions: { zh: '权限', en: 'Permissions' },
    status: { zh: '状态', en: 'Status' },
    createdAt: { zh: '创建时间', en: 'Created' },
    pending: { zh: '审批中', en: 'Pending' },
    approved: { zh: '已批准', en: 'Approved' },
    rejected: { zh: '已驳回', en: 'Rejected' },
    applied: { zh: '已应用', en: 'Applied' },
    revoked: { zh: '已撤销', en: 'Revoked' },
    failed: { zh: '失败', en: 'Failed' },
    autoApproved: { zh: '自动批准', en: 'Auto-approved' },
    noPermissions: { zh: '暂无权限', en: 'No permissions yet' },
    requestPermission: { zh: '申请新权限', en: 'Request New Permission' },
    reason: { zh: '申请理由', en: 'Reason' },
    low: { zh: '低', en: 'Low' },
    medium: { zh: '中', en: 'Medium' },
    high: { zh: '高', en: 'High' },
  }
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(msgs)) {
    out[k] = v[props.lang] || v.en
  }
  return out
})

function tr(key: string) {
  return t.value[key] || key
}

function statusClass(s: string) {
  const map: Record<string, string> = {
    pending: 'badge-warn',
    'auto-approved': 'badge-success',
    approved: 'badge-success',
    applied: 'badge-success',
    rejected: 'badge-danger',
    revoked: 'badge-muted',
    failed: 'badge-danger',
  }
  return map[s] || 'badge-muted'
}

function statusLabel(s: string) {
  const map: Record<string, string> = {
    pending: tr('pending'),
    'auto-approved': tr('autoApproved'),
    approved: tr('approved'),
    applied: tr('applied'),
    rejected: tr('rejected'),
    revoked: tr('revoked'),
    failed: tr('failed'),
  }
  return map[s] || s
}
</script>

<template>
  <div class="panel">
    <div class="section-head">
      <div>
        <h2>{{ tr('myPermissionsTitle') }}</h2>
      </div>
      <button class="primary" @click="emit('navigateRequest')">{{ tr('requestPermission') }}</button>
    </div>

    <div v-if="myPermissions.namespaces.length===0 && myPermissions.tools.length===0" class="muted">{{ tr('noPermissions') }}</div>
    <template v-else>
      <div v-if="myPermissions.namespaces.length > 0" class="subsection-title">{{ tr('namespace') }} ({{ myPermissions.namespaces.length }})</div>
      <div class="card" v-for="ns in myPermissions.namespaces" :key="ns.name">
        <div class="card-header">
          <span class="strong">{{ ns.name }}</span>
          <span class="badge" :class="statusClass(ns.access==='high'?'pending':'auto-approved')">{{ ns.access }}</span>
        </div>
        <div class="card-body">
          <div class="muted">{{ ns.resources.join(', ') }}</div>
          <div class="small muted">{{ ns.source.templateId }} · {{ new Date(ns.source.requestedAt).toLocaleString() }}</div>
        </div>
      </div>

      <div v-if="myPermissions.tools.length > 0" class="subsection-title">{{ tr('tools') }} ({{ myPermissions.tools.length }})</div>
      <div class="card" v-for="tool in myPermissions.tools" :key="tool.name + tool.tool">
        <div class="card-header">
          <span class="strong">{{ tool.tool.toUpperCase() }}</span>
          <span class="muted">{{ tool.name }}</span>
        </div>
        <div class="card-body">
          <div class="muted">{{ tool.permissions.join(', ') }}</div>
          <div v-if="tool.namespace" class="small muted">ns: {{ tool.namespace }}</div>
        </div>
      </div>
    </template>

    <div class="subsection-title">{{ tr('status') }}</div>
    <div class="card" v-for="req in requests" :key="req.id">
      <div class="card-header">
        <span>{{ req.templateId }}</span>
        <span class="badge" :class="statusClass(req.status)">{{ statusLabel(req.status) }}</span>
      </div>
      <div class="card-body">
        <div class="small muted">{{ tr('reason') }}: {{ req.reason || '-' }}</div>
        <div class="small muted">{{ new Date(req.createdAt).toLocaleString() }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel { padding: 16px; }
.section-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; margin-bottom: 10px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.badge { padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
.badge-warn { background: #fef3c7; color: #92400e; }
.badge-success { background: #d1fae5; color: #065f46; }
.badge-danger { background: #fee2e2; color: #991b1b; }
.badge-muted { background: #f3f4f6; color: #6b7280; }
.strong { font-weight: 700; }
.subsection-title { font-size: 14px; font-weight: 700; color: #374151; margin: 16px 0 8px; }
.muted { color: #6b7280; }
.small { font-size: 12px; }
</style>
