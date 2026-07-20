<script setup lang="ts">
import { onMounted, nextTick } from "vue";
import {
  Quit,
  WindowMinimise,
  WindowToggleMaximise,
  WindowShow,
} from "../wailsjs/runtime/runtime";
import { ui, api, go, lockNow, loadSettings, bumpZoom, resetZoom } from "./store";
import { initUIBridge } from "./uibridge";
import Toasts from "./Toasts.vue";
import UnlockView from "./views/UnlockView.vue";
import VaultView from "./views/VaultView.vue";
import SettingsView from "./views/SettingsView.vue";
import ApiServerView from "./views/ApiServerView.vue";
import MasterPasswordView from "./views/MasterPasswordView.vue";
import CategoriesView from "./views/CategoriesView.vue";

// Ctrl/Cmd +/-/0 zooms the app content (family pattern from go-notepad).
// preventDefault stops the webview from zooming the page itself — the title
// bar must never scale.
function onKey(e: KeyboardEvent) {
  if (!(e.ctrlKey || e.metaKey)) return;
  const k = e.key;
  if (k === "=" || k === "+") {
    e.preventDefault();
    bumpZoom(+1);
  } else if (k === "-" || k === "_") {
    e.preventDefault();
    bumpZoom(-1);
  } else if (k === "0") {
    e.preventDefault();
    resetZoom();
  }
}

// Ctrl/Cmd + mouse wheel zooms too (up = bigger), like every browser/editor.
function onWheel(e: WheelEvent) {
  if (!(e.ctrlKey || e.metaKey)) return;
  e.preventDefault();
  bumpZoom(e.deltaY < 0 ? +1 : -1);
}

onMounted(async () => {
  // Apply the persisted theme BEFORE the window is shown (StartHidden), so
  // the user never sees a default-theme flash (family pattern).
  await loadSettings();
  initUIBridge();
  window.addEventListener("keydown", onKey);
  // passive:false so preventDefault can stop the webview's page zoom.
  window.addEventListener("wheel", onWheel, { passive: false });
  await nextTick();
  WindowShow();
});
</script>

<template>
  <div class="window">
    <header class="titlebar" data-testid="titlebar" style="--wails-draggable: drag">
      <div class="brand">
        <!-- Lucide "key-round" (ISC) — the app's identity glyph per go-design. -->
        <svg
          class="app-glyph"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path
            d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"
          />
          <circle cx="16.5" cy="7.5" r=".5" fill="currentColor" />
        </svg>
        <span class="titlebar-title">go-passwords</span>
      </div>

      <div class="win-controls" style="--wails-draggable: no-drag">
        <!-- Shown only while the REST server has a port open — a password
             manager listening on the network is something the user should
             always be able to see at a glance. -->
        <button
          v-if="api.running"
          class="win-btn"
          :title="`REST API server is running on port ${api.port} — click to configure`"
          data-testid="api-indicator"
          @click="go('api')"
        >
          <span class="api-dot" aria-hidden="true"></span>
        </button>
        <button
          v-if="ui.unlocked"
          class="win-btn"
          title="Lock vault"
          data-testid="lock-now"
          @click="lockNow"
        >
          <!-- Lucide "lock" -->
          <svg
            class="ctrl-icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
        </button>
        <button
          class="win-btn"
          title="Settings"
          data-testid="open-settings"
          @click="go('settings')"
        >
          <!-- Lucide "settings" -->
          <svg
            class="ctrl-icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path
              d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"
            />
            <circle cx="12" cy="12" r="3" />
          </svg>
        </button>
        <button
          class="win-btn"
          title="Minimize"
          data-testid="window-minimize"
          @click="WindowMinimise"
        >
          &#x2013;
        </button>
        <button
          class="win-btn"
          title="Maximize"
          data-testid="window-maximize"
          @click="WindowToggleMaximise"
        >
          <svg class="ctrl-icon sq" viewBox="0 0 48 48" fill="currentColor" aria-hidden="true">
            <path
              d="M6 11.25C6 8.35051 8.3505 6 11.25 6H36.75C39.6495 6 42 8.3505 42 11.25V36.75C42 39.6495 39.6495 42 36.75 42H11.25C8.35051 42 6 39.6495 6 36.75V11.25ZM11.25 8.5C9.73122 8.5 8.5 9.73122 8.5 11.25V36.75C8.5 38.2688 9.73122 39.5 11.25 39.5H36.75C38.2688 39.5 39.5 38.2688 39.5 36.75V11.25C39.5 9.73122 38.2688 8.5 36.75 8.5H11.25Z"
            />
          </svg>
        </button>
        <button class="win-btn close" title="Close" data-testid="window-close" @click="Quit">
          &#x2715;
        </button>
      </div>
    </header>

    <main class="zoom-host">
      <UnlockView v-if="ui.view === 'unlock'" />
      <VaultView v-else-if="ui.view === 'vault'" />
      <SettingsView v-else-if="ui.view === 'settings'" />
      <ApiServerView v-else-if="ui.view === 'api'" />
      <MasterPasswordView v-else-if="ui.view === 'masterpassword'" />
      <CategoriesView v-else-if="ui.view === 'categories'" />
    </main>

    <Toasts />
  </div>
</template>
