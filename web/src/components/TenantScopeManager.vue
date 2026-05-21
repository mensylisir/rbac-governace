<script setup lang="ts">
import type { Tenant } from '../App.vue'

const props = defineProps<{
  tenants: Tenant[]
  canAdmin: boolean
  lang: string
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'open-create-modal': []
}>()
</script>

<template>
  <section v-if="canAdmin" class="panel">
    <div class="section-head">
      <div>
        <h2>{{ t.tenantScope }}</h2>
        <p>{{ lang === 'zh' ? '页面访问范围和租户同步权限是两层能力：这里管理页面可见范围，上方管理 Argo CD 租户同步权限。' : 'UI access scope and tenant sync permissions are separate layers. This section controls visibility; the form above controls Argo CD tenant sync permissions.' }}</p>
      </div>
      <button class="primary" @click="emit('open-create-modal')">{{ t.createTenant }}</button>
    </div>
    <div class="grid three">
      <article v-for="tenant in tenants" :key="tenant.id" class="card">
        <div class="row"><strong>{{ tenant.name }}</strong><span class="badge low">{{ tenant.id }}</span></div>
        <div class="small muted">{{ t.tenantClusters }}: {{ tenant.clusterIds.join(', ') || '-' }}</div>
        <div class="small muted">{{ t.tenantNamespaces }}: {{ tenant.namespaces.join(', ') || '-' }}</div>
      </article>
    </div>
  </section>
</template>
