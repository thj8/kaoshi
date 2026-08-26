<template>
  <transition-group name="toast" tag="div" class="toast-host">
    <div v-for="t in items" :key="t.id" class="toast" :class="t.type">{{ t.msg }}</div>
  </transition-group>
</template>

<script setup lang="ts">
import { items } from '../toast'
</script>

<style scoped>
.toast-host {
  position: fixed;
  top: calc(16px + env(safe-area-inset-top));
  left: 50%;
  transform: translateX(-50%);
  z-index: 1000;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  pointer-events: none;
}
.toast {
  max-width: min(86vw, 420px);
  padding: 11px 20px;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 500;
  color: #fff;
  background: rgba(29, 29, 31, 0.88);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: pre-wrap;
}
.toast::before {
  content: '';
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.toast.error {
  background: rgba(255, 235, 232, 0.95);
  border: 1px solid #ff8f86;
  color: #c0392b;
}
.toast.error::before { background: #e0404f; }
.toast.success::before { background: #4cd964; }
.toast.info::before { background: #6c9bff; }

.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s cubic-bezier(0.2, 0.9, 0.3, 1);
}
.toast-enter-from {
  opacity: 0;
  transform: translateY(-14px) scale(0.95);
}
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.95);
}
</style>
