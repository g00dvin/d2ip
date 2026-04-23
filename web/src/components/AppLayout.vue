<script setup lang="ts">
import { ref } from 'vue'
import SidebarNav from './SidebarNav.vue'
import ToastNotification from './ToastNotification.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const sidebarOpen = ref(false)

function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
}

function closeSidebar() {
  sidebarOpen.value = false
}
</script>

<template>
  <div class="flex h-screen overflow-hidden">
    <SidebarNav :open="sidebarOpen" @close="closeSidebar" @navigate="closeSidebar" />
    <main class="flex-1 overflow-y-auto p-6 max-md:p-3">
      <button
        class="md:hidden fixed top-3 left-3 z-50 btn btn-accent"
        @click="toggleSidebar"
      >
        ☰
      </button>
      <div class="md:hidden h-8" />
      <router-view />
    </main>
  </div>
  <ToastNotification />
  <ConfirmDialog />
</template>