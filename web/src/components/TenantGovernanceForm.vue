<script setup lang="ts">
import type { Template } from '../App.vue'

const props = defineProps<{
  clusterId: string
  tenantTemplates: Template[]
  selectedTenantTemplateId: string
  params: Record<string, string>
  renderedYaml: string
  warnings: string[]
  lang: string
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'template-change': [templateId: string]
  'preview': []
  'create-plan': []
  'update:params': [params: Record<string, string>]
}>()

function localizedTemplateName(template: Template) {
  return (props.t as Record<string, unknown>).localizedTemplateName
    ? ((props.t as Record<string, unknown>).localizedTemplateName as (t: Template) => string)(template)
    : template.name
}

function warningLabel(value: string) {
  if (props.lang === 'en') return value
  const warningText = (props.t as Record<string, unknown>).warningText as Record<string, string> | undefined
  if (value.includes('high-risk template') && warningText) return warningText.highRiskTemplate || value
  if (value.includes('risky RBAC binding') && warningText) return warningText.cleanupBindings || value
  return value
}

function updateParam(key: string, value: string) {
  emit('update:params', { ...props.params, [key]: value })
}
</script>

<template>
  <section class="panel">
    <div class="section-head">
      <div>
        <h2>{{ t.tenantGovernance }}</h2>
        <p>{{ t.tenantGovernanceHelp }}</p>
      </div>
    </div>
    <div class="stack">
      <label>{{ t.tenantTemplate }}
        <select :value="selectedTenantTemplateId" @change="emit('template-change', ($event.target as HTMLSelectElement).value)">
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
        <label>{{ t.tenantServiceAccount }} <input :value="params.serviceAccount" placeholder="team-a" @input="updateParam('serviceAccount', ($event.target as HTMLInputElement).value)" /><div class="small muted">{{ t.tenantServiceAccountHint }}</div></label>
        <label>{{ t.sourceRepo }} <input :value="params.sourceRepo" placeholder="*" @input="updateParam('sourceRepo', ($event.target as HTMLInputElement).value)" /></label>
      </div>
      <div v-if="selectedTenantTemplateId === 'argocd-static-tenant'" class="grid two">
        <label>{{ t.businessNamespace }} <input :value="params.targetNamespace" placeholder="team-a-prod" @input="updateParam('targetNamespace', ($event.target as HTMLInputElement).value)" /><div class="small muted">{{ t.businessNamespaceHint }}</div></label>
      </div>
      <div v-if="selectedTenantTemplateId === 'argocd-dynamic-tenant'" class="small muted">
        {{ lang === 'zh' ? '命名空间匹配规则和标签将自动从租户标识生成' : 'Namespace pattern and labels are auto-generated from the tenant ID' }}
      </div>
      <div class="row">
        <button :disabled="!selectedTenantTemplateId" @click="emit('preview')">{{ t.previewYaml }}</button>
        <button class="primary" :disabled="!selectedTenantTemplateId" @click="emit('create-plan')">{{ t.createPlan }}</button>
      </div>
      <div v-for="warning in warnings" :key="warning" class="finding medium"><strong>{{ t.warning }}</strong><div class="small">{{ warningLabel(warning) }}</div></div>
    </div>
  </section>

  <section class="panel governance-panel">
    <h2>{{ t.proposedYaml }}</h2>
    <p>{{ t.proposedYamlHelp }}</p>
    <pre>{{ renderedYaml || t.previewPlaceholder }}</pre>
  </section>
</template>
