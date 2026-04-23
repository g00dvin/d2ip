<script setup lang="ts">
import { useConfirm } from '@/composables/useConfirm'

const { visible: open, message, onOk: accept, onCancel: reject } = useConfirm()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[2000] flex items-center justify-center bg-black/50"
    >
      <div class="bg-surface-card border border-border p-6 min-w-80 max-w-md">
        <div class="mb-4 font-mono text-sm text-txt-primary">{{ message }}</div>
        <div class="flex gap-2 justify-end">
          <button class="btn btn-accent" @click="accept">Confirm</button>
          <button class="btn btn-danger" @click="reject">Cancel</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>