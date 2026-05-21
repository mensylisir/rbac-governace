<script setup lang="ts">
import type { Cluster, Template, Tenant } from '../App.vue'

const props = defineProps<{
  clusters: Cluster[]
  selectedClusterId: string
  tenantTemplates: Template[]
  selectedTenantTemplateId: string
  params: Record<string, string>
  renderedYaml: string
  warnings: string[]
  credentialNamespace: string
  credentialServiceAccount: string
  credentialExpiration: number
  credentialFormat: string
  tenantCredentialOutput: string
  tenantCredentialExpiresAt: string
  tenants: Tenant[]
  canAdmin: boolean
  lang: string
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'cluster-change': []
  'tenant-template-change': [templateId: string]
  'preview-tenant-plan': []
  'create-tenant-plan': []
  'create-tenant-credential': []
  'open-tenant-modal': []
  'update:credentialNamespace': [value: string]
  'update:credentialServiceAccount': [value: string]
  'update:credentialExpiration': [value: number]
  'update:credentialFormat': [value: string]
}>()

function localizedTemplateName(template: Template) {
  return (props.t as Record<string, unknown>).localizedTemplateName
    ? ((props.t as Record<string, unknown>).localizedTemplateName as (t: Template) => string)(template)
    : template.name
}

function formatTime(value?: string) {
  if (!value || value.startsWith('0001-')) return '-'
  return new Date(value).toLocaleString()
}

function warningLabel(value: string) {
  if (props.lang === 'en') return value
  const warningText = (props.t as Record<string, unknown>).warningText as Record<string, string> | undefined
  if (value.includes('high-risk template') && warningText) return warningText.highRiskTemplate || value
  if (value.includes('risky RBAC binding') && warningText) return warningText.cleanupBindings || value
  return value
}
</script>

<template>
  <section class="stack">
    <div class="toolbar">
      <label class="cluster-picker">{{ t.cluster }}
        <select :value="selectedClusterId" @change="emit('cluster-change')">
          <option v-for="cluster in clusters" :key="cluster.id" :value="cluster.id">{{ cluster.name }}</option>
        </select>
      </label>
      <button v-if="canAdmin" class="primary" @click="emit('open-tenant-modal')">{{ t.createTenant }}</button>
    </div>

    <div class="tool-layout">
      <section class="panel">
        <div class="section-head">
          <div>
            <h2>{{ t.tenantGovernance }}</h2>
            <p>{{ t.tenantGovernanceHelp }}</p>
          </div>
        </div>
        <div class="stack">
          <label>{{ t.tenantTemplate }}
            <select :value="selectedTenantTemplateId" @change="emit('tenant-template-change', ($event.target as HTMLSelectElement).value)">
              <option value="">{{ t.explicitTenantRequired }}</option>
              <option v-for="template in tenantTemplates" :key="template.id" :value="template.id">{{ localizedTemplateName(template) }}</option>
            </select>
          </label>
          <div class="small muted">{{ t.tenantControllerHint }}</div>
          <div v-if="selectedTenantTemplateId && params.namespace" class="small warning-hint">
            ⚠️ {{ (t.tenantSaLocationHint as string).replace('{namespace}', params.namespace) }}
          </div>
          <div v-if="!selectedTenantTemplateId" class="grid two">
            <label>{{ t.namespace }} <input :value="params.namespace" placeholder="scan Argo CD first" readonly /></label>
          </div>
          <div class="grid two">
            <label>{{ t.tenantServiceAccount }} <input :value="params.serviceAccount" placeholder="team-a" /><div class="small muted">{{ t.tenantServiceAccountHint }}</div></label>
            <label>{{ t.sourceRepo }} <input :value="params.sourceRepo" placeholder="*" /></label>
          </div>
          <div v-if="selectedTenantTemplateId === 'argocd-static-tenant'" class="grid two">
            <label>{{ t.businessNamespace }} <input :value="params.targetNamespace" placeholder="team-a-prod" /><div class="small muted">{{ t.businessNamespaceHint }}</div></label>
          </div>
          <div v-if="selectedTenantTemplateId === 'argocd-dynamic-tenant'" class="small muted">
            {{ lang === 'zh' ? '命名空间匹配规则和标签将自动从租户标识生成' : 'Namespace pattern and labels are auto-generated from the tenant ID' }}
          </div>
          <div class="row">
            <button :disabled="!selectedTenantTemplateId" @click="emit('preview-tenant-plan')">{{ t.previewYaml }}</button>
            <button class="primary" :disabled="!selectedTenantTemplateId" @click="emit('create-tenant-plan')">{{ t.createPlan }}</button>
          </div>
          <div v-for="warning in warnings" :key="warning" class="finding medium"><strong>{{ t.warning }}</strong><div class="small">{{ warningLabel(warning) }}</div></div>
        </div>
      </section>

      <section class="panel governance-panel">
        <h2>{{ t.proposedYaml }}</h2>
        <p>{{ t.proposedYamlHelp }}</p>
        <pre>{{ renderedYaml || t.previewPlaceholder }}</pre>
      </section>
    </div>

    <section class="panel">
      <div class="section-head">
        <div>
          <h2>{{ t.tenantCredential }}</h2>
          <p>{{ t.credentialHelp }}</p>
        </div>
      </div>
      <div class="grid two">
        <label>{{ t.credentialNamespace }} <input :value="credentialNamespace" @input="emit('update:credentialNamespace', ($event.target as HTMLInputElement).value)" placeholder="team-a" /></label>
        <label>{{ t.credentialServiceAccount }} <input :value="credentialServiceAccount" @input="emit('update:credentialServiceAccount', ($event.target as HTMLInputElement).value)" placeholder="team-a-deployer" /></label>
        <label>{{ t.credentialExpiration }} <input :value="credentialExpiration" type="number" min="600" max="86400" step="600" @input="emit('update:credentialExpiration', Number(($event.target as HTMLInputElement).value))" /></label>
        <label>{{ t.credentialFormat }}
          <select :value="credentialFormat" @change="emit('update:credentialFormat', ($event.target as HTMLSelectElement).value)">
            <option value="kubeconfig">kubeconfig</option>
            <option value="token">token</option>
          </select>
        </label>
      </div>
      <div class="row"><button class="primary" :disabled="!selectedClusterId || !credentialNamespace || !credentialServiceAccount" @click="emit('create-tenant-credential')">{{ t.generateCredential }}</button></div>
      <div v-if="tenantCredentialOutput" class="subsection">
        <div class="subsection-title">{{ t.credentialOutput }}</div>
        <p v-if="tenantCredentialExpiresAt">{{ t.credentialExpires }}: {{ formatTime(tenantCredentialExpiresAt) }}</p>
        <pre>{{ tenantCredentialOutput }}</pre>
      </div>
    </section>

    <section v-if="canAdmin" class="panel">
      <div class="section-head">
        <div>
          <h2>{{ t.tenantScope }}</h2>
          <p>{{ lang === 'zh' ? '页面访问范围和租户同步权限是两层能力：这里管理页面可见范围，上方管理 Argo CD 租户同步权限。' : 'UI access scope and tenant sync permissions are separate layers. This section controls visibility; the form above controls Argo CD tenant sync permissions.' }}</p>
        </div>
      </div>
      <div class="grid three">
        <article v-for="tenant in tenants" :key="tenant.id" class="card">
          <div class="row"><strong>{{ tenant.name }}</strong><span class="badge low">{{ tenant.id }}</span></div>
          <div class="small muted">{{ t.tenantClusters }}: {{ tenant.clusterIds.join(', ') || '-' }}</div>
          <div class="small muted">{{ t.tenantNamespaces }}: {{ tenant.namespaces.join(', ') || '-' }}</div>
        </article>
      </div>
    </section>
  </section>
</template>
