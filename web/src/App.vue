<template>
  <div class="app-container">
    <router-view />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useDeviceStore } from '@/stores/device'

const deviceStore = useDeviceStore()

onMounted(async () => {
  await deviceStore.initDevice()
})
</script>

<style>
.app-container {
  width: 100%;
  height: 100vh;
  overflow: hidden;
}
</style>