<script setup lang="ts">
const props = defineProps<{
  clusterId: string
  credentialNamespace: string
  credentialServiceAccount: string
  credentialExpiration: number
  credentialFormat: string
  tenantCredentialOutput: string
  tenantCredentialExpiresAt: string
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:credentialNamespace': [value: string]
  'update:credentialServiceAccount': [value: string]
  'update:credentialExpiration': [value: number]
  'update:credentialFormat': [value: string]
  'create-credential': []
}>()

function formatTime(value?: string) {
  if (!value || value.startsWith('0001-')) return '-'
  return new Date(value).toLocaleString()
}
</script>

<template>
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
    <div class="row"><button class="primary" :disabled="!clusterId || !credentialNamespace || !credentialServiceAccount" @click="emit('create-credential')">{{ t.generateCredential }}</button></div>
    <div v-if="tenantCredentialOutput" class="subsection">
      <div class="subsection-title">{{ t.credentialOutput }}</div>
      <p v-if="tenantCredentialExpiresAt">{{ t.credentialExpires }}: {{ formatTime(tenantCredentialExpiresAt) }}</p>
      <pre>{{ tenantCredentialOutput }}</pre>
    </div>
  </section>
</template>
