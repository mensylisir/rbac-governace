<script setup lang="ts">
import type { Cluster } from '../App.vue'

const props = defineProps<{
  clusters: Cluster[]
  selectedClusterId: string
}>()

const emit = defineEmits<{
  change: []
}>()
</script>

<template>
  <label class="cluster-picker">
    {{ $attrs['data-label'] || 'Cluster' }}
    <select :value="selectedClusterId" @change="emit('change')">
      <option v-for="cluster in clusters" :key="cluster.id" :value="cluster.id">{{ cluster.name }}</option>
    </select>
  </label>
</template>
