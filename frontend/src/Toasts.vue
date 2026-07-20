<script setup lang="ts">
import { onMounted } from "vue";
import { dismissToast, toast, toastState } from "./toast";

// Dev-only: F8 fires one toast of each severity so the palette can be
// reviewed live. Stripped from production builds.
onMounted(() => {
  if (import.meta.env.DEV) {
    window.addEventListener("keydown", (e) => {
      if (e.key === "F8") {
        toast("This is an info toast.");
        toast.success("Operation completed.");
        toast.warning("Careful — this file is cleartext.");
        toast.error("Something went wrong.");
      }
    });
  }
});
</script>

<template>
  <TransitionGroup name="toast" tag="div" class="toasts" data-testid="toasts">
    <div
      v-for="t in toastState.items"
      :key="t.id"
      class="toast-item"
      :class="t.kind"
      data-testid="toast-item"
    >
      <!-- Severity icon (Fluent InfoBar pattern): info / check / alert / x -->
      <svg
        v-if="t.kind === 'success'"
        class="toast-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="m9 12 2 2 4-4" />
      </svg>
      <svg
        v-else-if="t.kind === 'warning'"
        class="toast-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path
          d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"
        />
        <path d="M12 9v4" />
        <path d="M12 17h.01" />
      </svg>
      <svg
        v-else-if="t.kind === 'error'"
        class="toast-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="m15 9-6 6" />
        <path d="m9 9 6 6" />
      </svg>
      <svg
        v-else
        class="toast-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="M12 16v-4" />
        <path d="M12 8h.01" />
      </svg>

      <span class="toast-text">{{ t.text }}</span>
      <button class="toast-close" title="Close" @click="dismissToast(t.id)">✕</button>
    </div>
  </TransitionGroup>
</template>
