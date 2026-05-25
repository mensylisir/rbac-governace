<script setup lang="ts">
import { computed, ref, watch } from 'vue'

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
  clusters: Array<{ id: string; name: string }>
  templates: Array<{ id: string; tool: string; name: string; riskLevel: string; params: Array<{ name: string; label: string; required: boolean; default?: string }> }>
}>()

const emit = defineEmits<{
  submitRequest: [req: { templateId: string; clusterId: string; params: Record<string, string>; reason: string }]
}>()

const selectedTemplate = ref('')
const selectedCluster = ref('')
const formParams = ref<Record<string, string>>({})
const requestReason = ref('')
const submitted = ref(false)

const t = computed(() => {
  const msgs: Record<string, Record<string, string>> = {
    requestPermissionTitle: { zh: '申请权限', en: 'Request Permission' },
    selectTemplate: { zh: '选择权限类型', en: 'Select permission type' },
    selectCluster: { zh: '选择集群', en: 'Select cluster' },
    reason: { zh: '申请理由', en: 'Reason' },
    submit: { zh: '提交申请', en: 'Submit' },
    submitted: { zh: '申请已提交', en: 'Request Submitted' },
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

const templateParams = computed(() => {
  if (!selectedTemplate.value) return []
  const tmpl = props.templates.find(t => t.id === selectedTemplate.value)
  return tmpl?.params || []
})

watch(selectedTemplate, () => {
  formParams.value = {}
  const tmpl = props.templates.find(t => t.id === selectedTemplate.value)
  if (tmpl) {
    for (const p of tmpl.params) {
      if (p.default) formParams.value[p.name] = p.default
    }
  }
})

function tr(key: string) {
  return t.value[key] || key
}

function submit() {
  emit('submitRequest', {
    templateId: selectedTemplate.value,
    clusterId: selectedCluster.value,
    params: { ...formParams.value },
    reason: requestReason.value,
  })
  submitted.value = true
  selectedTemplate.value = ''
  selectedCluster.value = ''
  formParams.value = {}
  requestReason.value = ''
}
</script>

<template>
  <div class="panel">
    <h2>{{ tr('requestPermissionTitle') }}</h2>
    <div v-if="submitted" class="success-banner" @click="submitted = false">
      {{ tr('submitted') }} — <span class="click">{{ lang === 'zh' ? '点击继续申请' : 'Click to request another' }}</span>
    </div>
    <div class="form-section">
      <label>
        <span class="field-label">{{ tr('selectTemplate') }}</span>
        <select v-model="selectedTemplate">
          <option value="">-- {{ tr('selectTemplate') }} --</option>
          <option v-for="tmpl in templates" :key="tmpl.id" :value="tmpl.id">{{ tmpl.name }} ({{ tr(tmpl.riskLevel) }})</option>
        </select>
      </label>

      <label>
        <span class="field-label">{{ tr('selectCluster') }}</span>
        <select v-model="selectedCluster">
          <option value="">-- {{ tr('selectCluster') }} --</option>
          <option v-for="c in clusters" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
      </label>

      <div v-for="p in templateParams" :key="p.name" class="grid two">
        <label>
          <span class="field-label">{{ p.label }}</span>
          <input v-model="formParams[p.name]" :placeholder="p.default || ''" />
        </label>
      </div>

      <label>
        <span class="field-label">{{ tr('reason') }}</span>
        <textarea v-model="requestReason" rows="3" placeholder="Why do you need this permission?"></textarea>
      </label>

      <button class="primary" :disabled="!selectedTemplate || !selectedCluster" @click="submit">{{ tr('submit') }}</button>
    </div>
  </div>
</template>

<style scoped>
.panel { padding: 16px; }
.form-section { display: grid; gap: 14px; max-width: 600px; }
.field-label { display: block; font-size: 12px; font-weight: 600; color: #374151; margin-bottom: 4px; }
select, input, textarea { width: 100%; padding: 8px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; }
.success-banner { background: #d1fae5; color: #065f46; padding: 12px; border-radius: 8px; margin-bottom: 16px; cursor: pointer; }
.click { text-decoration: underline; }
.grid.two { display: grid; grid-template-columns: 1fr; }
button.primary { padding: 8px 16px; border-radius: 6px; border: none; background: #2563eb; color: #fff; font-weight: 600; cursor: pointer; }
button.primary:disabled { background: #93c5fd; cursor: not-allowed; }
</style>
