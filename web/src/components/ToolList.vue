<script setup lang="ts">
import type { Tool, Finding } from '../App.vue'

type FindingSeverity = Finding['severity']

const props = defineProps<{
  tools: Tool[]
  currentTool: Tool | null
  countSuffix: string
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'govern-tool': [tool: Tool]
}>()

function maxSeverity(findings: Finding[]): FindingSeverity {
  if (findings.some((f) => f.severity === 'high')) return 'high'
  if (findings.some((f) => f.severity === 'medium')) return 'medium'
  return 'low'
}

function severityLabel(severity: FindingSeverity) {
  return (props.t.severity as Record<string, string>)[severity] || severity
}

function findingTitle(finding: Finding) {
  return (props.t.findingTitle as Record<string, string>)[finding.ruleId] || finding.title
}

function findingDescription(finding: Finding) {
  return (props.t.findingDesc as Record<string, string>)[finding.ruleId] || finding.description
}
</script>

<template>
  <section class="panel tool-list">
    <div class="section-head">
      <div>
        <h2>{{ t.detectedTools }}</h2>
        <p>{{ tools.length }} {{ countSuffix }}{{ t.detectedTools }}</p>
      </div>
    </div>
    <article
      v-for="tool in tools"
      :key="tool.id"
      class="tool-row"
      :class="{ active: currentTool?.id === tool.id }"
      @click="emit('govern-tool', tool)"
    >
      <div class="tool-main">
        <div class="row">
          <div class="card-title">{{ tool.name }}</div>
          <span class="badge">{{ tool.type === 'argocd' ? 'Argo CD' : tool.type }}</span>
          <span class="badge" :class="maxSeverity(tool.findings)">{{ severityLabel(maxSeverity(tool.findings)) }}</span>
        </div>
        <div class="meta-line">
          <span>{{ t.namespace }}: <strong>{{ tool.namespace }}</strong></span>
          <span>{{ t.kind }}: <strong>{{ tool.kind }}</strong></span>
          <span>{{ t.serviceAccount }}: <strong class="mono">{{ tool.serviceAccount }}</strong></span>
        </div>
        <div class="finding-list">
          <div v-for="finding in tool.findings" :key="finding.id" class="finding compact" :class="finding.severity">
            <div class="row"><strong>{{ findingTitle(finding) }}</strong><span class="badge" :class="finding.severity">{{ severityLabel(finding.severity) }}</span></div>
            <div class="small muted">{{ findingDescription(finding) }}</div>
            <div class="small mono">{{ finding.resource }}</div>
          </div>
        </div>
      </div>
      <button class="primary" @click.stop="emit('govern-tool', tool)">{{ t.govern }}</button>
    </article>
    <div v-if="!tools.length" class="empty">{{ t.noTools }}</div>
  </section>
</template>
