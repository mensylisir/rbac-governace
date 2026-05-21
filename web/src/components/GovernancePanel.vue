<script setup lang="ts">
import type { Tool, Template } from '../App.vue'

const props = defineProps<{
  currentTool: Tool | null
  candidateTemplates: Template[]
  hasToolTemplate: boolean
  selectedTemplateId: string
  selectedTemplateParams: Template['params']
  cleanupOldBindings: boolean
  renderedYaml: string
  warnings: string[]
  t: Record<string, unknown>
  params: Record<string, string>
}>()

const emit = defineEmits<{
  'template-change': []
  'preview': []
  'create-plan': []
  'quick-credential': [tool: Tool]
  'update:cleanupOldBindings': [value: boolean]
}>()

function localizedTemplateName(template: Template) {
  return (props.t as Record<string, unknown>).localizedTemplateName
    ? ((props.t as Record<string, unknown>).localizedTemplateName as (t: Template) => string)(template)
    : template.name
}

function localizedParamLabel(param: { name: string; label: string }) {
  return (props.t as Record<string, unknown>).localizedParamLabel
    ? ((props.t as Record<string, unknown>).localizedParamLabel as (p: { name: string; label: string }) => string)(param)
    : param.label || param.name
}

function cleanupCandidates(tool: Tool | null): string[] {
  if (!tool) return []
  const seen = new Set<string>()
  const out: string[] = []
  const eligibleRules = ['cluster-admin', 'wildcard-rbac', 'cluster-write', 'pod-exec', 'privilege-escalation', 'argocd-controller-cluster-admin']
  for (const finding of tool.findings) {
    const isEligible = eligibleRules.includes(finding.ruleId)
    const isBinding = /^ClusterRoleBinding\/[^/]+$/.test(finding.resource) || /^[^/]+\/RoleBinding\/[^/]+$/.test(finding.resource)
    if (!isEligible || !isBinding || seen.has(finding.resource)) continue
    seen.add(finding.resource)
    out.push(finding.resource)
  }
  return out
}

function warningLabel(value: string) {
  const lang = (props.t as Record<string, unknown>).lang as string | undefined
  if (lang === 'en') return value
  const warningText = (props.t as Record<string, unknown>).warningText as Record<string, string> | undefined
  if (value.includes('high-risk template') && warningText) return warningText.highRiskTemplate || value
  if (value.includes('risky RBAC binding') && warningText) return warningText.cleanupBindings || value
  return value
}
</script>

<template>
  <section class="panel governance-panel">
    <h2>{{ t.governanceAction }}</h2>
    <div v-if="currentTool" class="stack">
      <div class="kv">
        <span>{{ t.tool }}</span><span>{{ currentTool.name }}</span>
        <span>{{ t.serviceAccount }}</span><span class="mono">{{ currentTool.namespace }}/{{ currentTool.serviceAccount }}</span>
      </div>
      <label>
        {{ t.template }}
        <select :value="selectedTemplateId" @change="emit('template-change')">
          <option v-if="!hasToolTemplate" value="">{{ t.noTemplatesForTool }}</option>
          <option v-for="template in candidateTemplates" :key="template.id" :value="template.id">
            {{ localizedTemplateName(template) }}
          </option>
        </select>
      </label>
      <div class="subsection">
        <div class="subsection-title">{{ t.templateParameters }}</div>
        <div class="grid two">
          <label v-for="param in selectedTemplateParams" :key="param.name">
            {{ localizedParamLabel(param) }}
            <input :value="params[param.name]" @input="emit('template-change')" />
          </label>
        </div>
      </div>
      <div v-if="!hasToolTemplate" class="small muted">{{ t.noTemplatesForTool }}</div>
      <label class="check-row">
        <input
          :checked="cleanupOldBindings"
          type="checkbox"
          @change="emit('update:cleanupOldBindings', ($event.target as HTMLInputElement).checked)"
        />
        <span><strong>{{ t.cleanupOldBindings }}</strong><small>{{ t.cleanupOldBindingsHelp }}</small></span>
      </label>
      <div class="cleanup-list">
        <div class="subsection-title">{{ t.cleanupCandidates }}</div>
        <div v-if="cleanupCandidates(currentTool).length" class="pill-row">
          <span v-for="item in cleanupCandidates(currentTool)" :key="item" class="badge high mono">{{ item }}</span>
        </div>
        <div v-else class="small muted">{{ t.noCleanupBindings }}</div>
      </div>
      <div class="row">
        <button :disabled="!hasToolTemplate" @click="emit('preview')">{{ t.previewYaml }}</button>
        <button class="primary" :disabled="!hasToolTemplate" @click="emit('create-plan')">{{ t.createPlan }}</button>
        <button @click="emit('quick-credential', currentTool)">{{ t.quickCredential }}</button>
      </div>
      <div v-for="warning in warnings" :key="warning" class="finding medium">
        <strong>{{ t.warning }}</strong><div class="small">{{ warningLabel(warning) }}</div>
      </div>
      <div class="subsection">
        <div class="subsection-title">{{ t.proposedYaml }}</div>
        <p>{{ t.proposedYamlHelp }}</p>
        <pre>{{ renderedYaml || t.previewPlaceholder }}</pre>
      </div>
    </div>
    <div v-else class="empty">{{ t.selectTool }}</div>
  </section>
</template>
