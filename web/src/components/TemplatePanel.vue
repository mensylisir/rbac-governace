<script setup lang="ts">
import type { Template } from '../App.vue'

const props = defineProps<{
  builtinTemplates: Template[]
  customTemplates: Template[]
  canAdmin: boolean
  scopeText: Record<string, string>
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'open-template-modal': []
  'open-template-preview': [template: Template]
}>()

function localizedTemplateName(template: Template) {
  return (props.t as Record<string, unknown>).localizedTemplateName
    ? ((props.t as Record<string, unknown>).localizedTemplateName as (t: Template) => string)(template)
    : template.name
}

function localizedTemplateDescription(template: Template) {
  return (props.t as Record<string, unknown>).localizedTemplateDescription
    ? ((props.t as Record<string, unknown>).localizedTemplateDescription as (t: Template) => string)(template)
    : template.description || ''
}

function permissionProfileLabel(template: Template) {
  return (props.t as Record<string, unknown>).permissionProfileLabel
    ? ((props.t as Record<string, unknown>).permissionProfileLabel as (t: Template) => string)(template)
    : template.riskLevel
}
</script>

<template>
  <section class="stack">
    <section class="panel template-catalog-panel">
      <div class="section-head">
        <div>
          <h2>{{ t.templateCatalog }}</h2>
          <p>{{ t.templateCatalogHelp }}</p>
        </div>
        <button v-if="canAdmin" class="primary" @click="emit('open-template-modal')">{{ t.createTemplate }}</button>
      </div>
      <div class="template-summary-grid">
        <div class="template-summary"><span>{{ t.builtinTemplates }}</span><strong>{{ builtinTemplates.length }}</strong></div>
        <div class="template-summary"><span>{{ t.customTemplates }}</span><strong>{{ customTemplates.length }}</strong></div>
      </div>
      <div class="template-table">
        <div class="template-section-title">{{ t.builtinTemplates }}</div>
        <article v-for="template in builtinTemplates" :key="template.id" class="template-row">
          <div>
            <div class="card-title">{{ template.tool }} · {{ permissionProfileLabel(template) }} — {{ localizedTemplateName(template) }}</div>
            <p>{{ localizedTemplateDescription(template) }}</p>
            <div class="small muted mono">{{ template.id }}</div>
          </div>
          <div class="template-meta">
            <span class="badge">{{ template.tool }}</span>
            <span class="badge">{{ permissionProfileLabel(template) }}</span>
            <span class="badge">{{ scopeText[template.scope] || template.scope }}</span>
          </div>
          <button @click="emit('open-template-preview', template)">{{ t.preview }}</button>
        </article>
        <div class="template-section-title">{{ t.customTemplates }}</div>
        <article v-for="template in customTemplates" :key="template.id" class="template-row">
          <div>
            <div class="card-title">{{ template.tool }} · {{ permissionProfileLabel(template) }} — {{ localizedTemplateName(template) }}</div>
            <p>{{ localizedTemplateDescription(template) }}</p>
            <div class="small muted mono">{{ template.id }}</div>
          </div>
          <div class="template-meta">
            <span class="badge">{{ template.tool }}</span>
            <span class="badge">{{ permissionProfileLabel(template) }}</span>
            <span class="badge">{{ scopeText[template.scope] || template.scope }}</span>
          </div>
          <button @click="emit('open-template-preview', template)">{{ t.preview }}</button>
        </article>
        <div v-if="!customTemplates.length" class="empty compact-empty">{{ t.noCustomTemplates }}</div>
      </div>
    </section>
  </section>
</template>
