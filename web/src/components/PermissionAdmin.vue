<script setup lang="ts">
import { computed } from 'vue'

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

const props = defineProps<{
  lang: 'zh' | 'en'
  isAdmin: boolean
  permissionRequests: PermissionRequest[]
}>()

const emit = defineEmits<{
  refresh: []
  approveRequest: [id: string]
  rejectRequest: [id: string, reason: string]
  revokeRequest: [id: string]
}>()

const requests = computed(() => props.permissionRequests)

const pendingRequests = computed(() => requests.value.filter(r => r.status === 'pending'))

const t = computed(() => {
  const msgs: Record<string, Record<string, string>> = {
    approvalQueueTitle: { zh: '权限审批', en: 'Approval Queue' },
    tabQueue: { zh: '审批队列', en: 'Queue' },
    requester: { zh: '申请人', en: 'Requester' },
    template: { zh: '模板', en: 'Template' },
    cluster: { zh: '集群', en: 'Cluster' },
    riskLevel: { zh: '风险等级', en: 'Risk' },
    reason: { zh: '申请理由', en: 'Reason' },
    createdAt: { zh: '创建时间', en: 'Created' },
    action: { zh: '操作', en: 'Action' },
    approve: { zh: '批准', en: 'Approve' },
    reject: { zh: '驳回', en: 'Reject' },
    revoke: { zh: '撤销', en: 'Revoke' },
    refresh: { zh: '刷新', en: 'Refresh' },
    noPending: { zh: '暂无待审批申请', en: 'No pending requests' },
    allRequests: { zh: '全部申请', en: 'All Requests' },
    status: { zh: '状态', en: 'Status' },
    pending: { zh: '审批中', en: 'Pending' },
    approved: { zh: '已批准', en: 'Approved' },
    rejected: { zh: '已驳回', en: 'Rejected' },
    applied: { zh: '已应用', en: 'Applied' },
    revoked: { zh: '已撤销', en: 'Revoked' },
    failed: { zh: '失败', en: 'Failed' },
    autoApproved: { zh: '自动批准', en: 'Auto-approved' },
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

function canApprove(r: PermissionRequest) {
  return props.isAdmin && r.status === 'pending'
}

function canRevoke(r: PermissionRequest) {
  return props.isAdmin && r.status === 'applied'
}
</script>

<template>
  <div class="panel">
    <div class="section-head">
      <h2>{{ tr('approvalQueueTitle') }}</h2>
      <button class="btn-sm" @click="emit('refresh')">{{ tr('refresh') }}</button>
    </div>

    <div v-if="pendingRequests.length===0" class="muted">{{ tr('noPending') }}</div>
    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ tr('requester') }}</th>
          <th>{{ tr('template') }}</th>
          <th>{{ tr('cluster') }}</th>
          <th>{{ tr('riskLevel') }}</th>
          <th>{{ tr('reason') }}</th>
          <th>{{ tr('createdAt') }}</th>
          <th>{{ tr('action') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="req in pendingRequests" :key="req.id">
          <td>{{ req.requesterId }}</td>
          <td>{{ req.templateId }}</td>
          <td>{{ req.clusterId }}</td>
          <td><span class="badge" :class="statusClass(req.riskLevel==='high'?'pending':'auto-approved')">{{ tr(req.riskLevel) }}</span></td>
          <td class="muted">{{ req.reason || '-' }}</td>
          <td class="muted">{{ new Date(req.createdAt).toLocaleString() }}</td>
          <td>
            <button class="btn-sm success" @click="emit('approveRequest', req.id)">{{ tr('approve') }}</button>
            <button class="btn-sm danger" @click="emit('rejectRequest', req.id, 'rejected by admin')">{{ tr('reject') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <h3>{{ tr('allRequests') }}</h3>
    <div class="card" v-for="req in requests" :key="req.id">
      <div class="card-header">
        <span>{{ req.requesterId }} → {{ req.templateId }}</span>
        <span class="badge" :class="statusClass(req.status)">{{ statusLabel(req.status) }}</span>
      </div>
      <div class="card-body">
        <div class="small muted">{{ req.reason || '-' }} · {{ new Date(req.createdAt).toLocaleString() }}</div>
        <button v-if="canRevoke(req)" class="btn-sm danger" @click="emit('revokeRequest', req.id)">{{ tr('revoke') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel { padding: 16px; }
.section-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; margin-bottom: 10px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.card-body { display: flex; justify-content: space-between; align-items: center; }
.badge { padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
.badge-warn { background: #fef3c7; color: #92400e; }
.badge-success { background: #d1fae5; color: #065f46; }
.badge-danger { background: #fee2e2; color: #991b1b; }
.badge-muted { background: #f3f4f6; color: #6b7280; }
.data-table { width: 100%; border-collapse: collapse; font-size: 13px; margin-bottom: 16px; }
.data-table th, .data-table td { padding: 8px; border-bottom: 1px solid #e5e7eb; text-align: left; }
.data-table th { font-weight: 700; color: #374151; }
.btn-sm { padding: 4px 10px; font-size: 12px; border-radius: 4px; cursor: pointer; border: 1px solid; margin-right: 4px; }
.success { background: #d1fae5; border-color: #a7f3d0; color: #065f46; }
.danger { background: #fee2e2; border-color: #fecaca; color: #991b1b; }
.muted { color: #6b7280; }
.small { font-size: 12px; }
</style>
