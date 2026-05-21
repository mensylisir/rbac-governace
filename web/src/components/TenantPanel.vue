<script setup lang="ts">
import { ref } from 'vue'
import type { Cluster, Template, Tenant } from '../App.vue'
import ClusterSelector from './ClusterSelector.vue'
import TenantGovernanceForm from './TenantGovernanceForm.vue'
import TenantCredentialGenerator from './TenantCredentialGenerator.vue'
import TenantScopeManager from './TenantScopeManager.vue'

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
  'update:params': [params: Record<string, string>]
}>()

const activeTab = ref('governance')

function tabLabel(key: string) {
  const labels = props.t.tabLabels as Record<string, string> | undefined
  return labels?.[key] || key
}
</script>

<template>
  <section class="stack">
    <div class="toolbar">
      <ClusterSelector
        :clusters="clusters"
        :selected-cluster-id="selectedClusterId"
        :data-label="t.cluster"
        @change="emit('cluster-change')"
      />
    </div>

    <div class="tabs">
      <button
        v-for="tab in ['governance', 'credential', 'scope']"
        :key="tab"
        :class="['tab-btn', { active: activeTab === tab }]"
        @click="activeTab = tab"
      >
        {{ tabLabel(tab) }}
      </button>
    </div>

    <div v-if="activeTab === 'governance'" class="tool-layout">
      <TenantGovernanceForm
        :cluster-id="selectedClusterId"
        :tenant-templates="tenantTemplates"
        :selected-tenant-template-id="selectedTenantTemplateId"
        :params="params"
        :rendered-yaml="renderedYaml"
        :warnings="warnings"
        :lang="lang"
        :t="t"
        @template-change="emit('tenant-template-change', $event)"
        @preview="emit('preview-tenant-plan')"
        @create-plan="emit('create-tenant-plan')"
        @update:params="(p) => emit('update:params', p)"
      />
    </div>

    <div v-if="activeTab === 'credential'">
      <TenantCredentialGenerator
        :cluster-id="selectedClusterId"
        :credential-namespace="credentialNamespace"
        :credential-service-account="credentialServiceAccount"
        :credential-expiration="credentialExpiration"
        :credential-format="credentialFormat"
        :tenant-credential-output="tenantCredentialOutput"
        :tenant-credential-expires-at="tenantCredentialExpiresAt"
        :t="t"
        @update:credential-namespace="(v) => emit('update:credentialNamespace', v)"
        @update:credential-service-account="(v) => emit('update:credentialServiceAccount', v)"
        @update:credential-expiration="(v) => emit('update:credentialExpiration', v)"
        @update:credential-format="(v) => emit('update:credentialFormat', v)"
        @create-credential="emit('create-tenant-credential')"
      />
    </div>

    <div v-if="activeTab === 'scope'">
      <TenantScopeManager
        :tenants="tenants"
        :can-admin="canAdmin"
        :lang="lang"
        :t="t"
        @open-create-modal="emit('open-tenant-modal')"
      />
    </div>
  </section>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--cds-border-subtle, #e0e0e0);
  margin-bottom: 1rem;
}

.tab-btn {
  padding: 0.75rem 1.25rem;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--cds-text-secondary, #525252);
  transition: all 0.15s ease;
}

.tab-btn:hover {
  color: var(--cds-text-primary, #161616);
  background: var(--cds-layer-hover, #e8e8e8);
}

.tab-btn.active {
  color: var(--cds-text-primary, #161616);
  border-bottom-color: var(--cds-border-interactive, #0f62fe);
  font-weight: 600;
}
</style>
