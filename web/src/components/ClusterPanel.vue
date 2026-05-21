<script setup lang="ts">
import type { Cluster, Me } from '../App.vue'

const props = defineProps<{
  clusters: Cluster[]
  canAdmin: boolean
  me: Me | null
  importForm: { name: string; kubeconfig: string }
  t: Record<string, unknown>
  lang: string
}>()

const emit = defineEmits<{
  'import-cluster': []
  'import-in-cluster': []
  'test-cluster': [id: string]
  'scan-cluster': [id: string]
  'open-tools': [cluster: Cluster]
  'update:importForm': [value: { name: string; kubeconfig: string }]
}>()

function statusLabel(value?: string) {
  if (!value) return '-'
  return (props.t.statusText as Record<string, string>)[value] || value
}

function messageLabel(value?: string) {
  if (!value) return '-'
  if (props.lang === 'en') return value
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

function formatTime(value?: string) {
  if (!value || value.startsWith('0001-')) return '-'
  return new Date(value).toLocaleString()
}
</script>

<template>
  <section v-if="canAdmin" class="grid" style="margin-bottom: 14px">
    <section class="panel">
      <h2>{{ t.currentUser }}</h2>
      <div class="kv"><span>{{ t.user }}</span><span>{{ me?.name || '-' }}</span><span>{{ t.role }}</span><span>{{ me?.role || '-' }}</span></div>
    </section>
  </section>

  <section class="grid two">
    <section class="panel">
      <h2>{{ t.importCluster }}</h2>
      <div class="stack">
        <label>{{ t.name }} <input :value="importForm.name" @input="emit('update:importForm', { ...importForm, name: ($event.target as HTMLInputElement).value })" placeholder="rbac-manager-test" /></label>
        <label>{{ t.kubeconfig }} <textarea :value="importForm.kubeconfig" @input="emit('update:importForm', { ...importForm, kubeconfig: ($event.target as HTMLTextAreaElement).value })" :placeholder="lang === 'zh' ? '粘贴 kubeconfig 内容' : 'Paste kubeconfig here'" /></label>
        <div class="row">
          <button class="primary" @click="emit('import-cluster')">{{ t.importAndTest }}</button>
          <button @click="emit('import-in-cluster')">{{ t.useInCluster }}</button>
        </div>
      </div>
    </section>

    <section class="panel">
      <h2>{{ t.knownClusters }}</h2>
      <div class="grid">
        <article v-for="cluster in clusters" :key="cluster.id" class="card">
          <div class="row">
            <div class="card-title">{{ cluster.name }}</div>
            <span class="badge" :class="cluster.status === 'connected' ? 'success' : 'high'">{{ statusLabel(cluster.status || 'unknown') }}</span>
            <span class="badge" :class="cluster.rbacManagerStatus === 'installed' ? 'success' : 'medium'">RBAC Manager {{ statusLabel(cluster.rbacManagerStatus || 'unknown') }}</span>
          </div>
          <div class="kv">
            <span>{{ t.context }}</span><span class="mono">{{ cluster.context || '-' }}</span>
            <span>{{ t.apiServer }}</span><span class="mono">{{ cluster.apiServer || '-' }}</span>
            <span>{{ t.lastScan }}</span><span>{{ formatTime(cluster.lastScanAt) }}</span>
            <span>{{ t.message }}</span><span>{{ messageLabel(cluster.message) }}</span>
          </div>
          <div class="row">
            <button @click="emit('open-tools', cluster)">{{ t.openTools }}</button>
            <button @click="emit('test-cluster', cluster.id)">{{ t.test }}</button>
            <button class="primary" @click="emit('scan-cluster', cluster.id)">{{ t.scan }}</button>
          </div>
        </article>
        <div v-if="!clusters.length" class="empty">{{ t.noClusters }}</div>
      </div>
    </section>
  </section>
</template>
