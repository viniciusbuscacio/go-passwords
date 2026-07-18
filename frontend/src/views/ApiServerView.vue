<script setup lang="ts">
import { onMounted, ref } from "vue";
import {
  AddAllowlistEntry,
  APIState,
  GetAPIKey,
  GetSettings,
  RemoveAllowlistEntry,
  RotateAPIKey,
  SetAPIAutoStart,
  ToggleServer,
} from "../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { back } from "../store";

const running = ref(false);
const url = ref("");
const autoStart = ref(false);
const allowlist = ref<string[]>([]);
const newIP = ref("");
const ipError = ref("");
const serverError = ref("");
const apiKey = ref("");
const keyRevealed = ref(false);
const copied = ref("");

async function refresh() {
  const st = await APIState();
  running.value = st.running;
  url.value = st.url;
  const cfg = await GetSettings();
  autoStart.value = cfg.apiAutoStart;
  allowlist.value = cfg.apiAllowlist ?? [];
  apiKey.value = await GetAPIKey();
}

onMounted(refresh);

async function toggle() {
  serverError.value = "";
  try {
    const st = await ToggleServer();
    running.value = st.running;
    url.value = st.url;
  } catch (e) {
    serverError.value = String(e);
    await refresh();
  }
}

async function toggleAutoStart() {
  autoStart.value = !autoStart.value;
  await SetAPIAutoStart(autoStart.value);
}

async function addIP() {
  ipError.value = "";
  if (!newIP.value.trim()) return;
  try {
    await AddAllowlistEntry(newIP.value.trim());
    newIP.value = "";
    await refresh();
  } catch (e) {
    ipError.value = String(e);
  }
}

async function removeIP(cidr: string) {
  await RemoveAllowlistEntry(cidr);
  await refresh();
}

async function rotate() {
  apiKey.value = await RotateAPIKey();
  keyRevealed.value = false;
}

async function copy(text: string, what: string) {
  await ClipboardSetText(text);
  copied.value = what;
  setTimeout(() => (copied.value = ""), 1500);
}

function maskedKey(): string {
  if (keyRevealed.value) return apiKey.value;
  return apiKey.value ? apiKey.value.slice(0, 6) + "…" + apiKey.value.slice(-4) : "";
}
</script>

<template>
  <div class="view">
    <div class="view-title">
      <button class="btn icon" data-testid="back" title="Back" @click="back">←</button>
      REST API Server
    </div>

    <div class="panel">
      <h3>Server</h3>
      <div class="setting-row">
        <span>
          Status:
          <strong data-testid="status">{{ running ? "Running" : "Stopped" }}</strong>
          <span v-if="running" class="hint mono" style="margin-left: 8px">{{ url }}</span>
        </span>
        <button class="btn primary" data-testid="toggle-server" @click="toggle">
          {{ running ? "Stop" : "Start" }}
        </button>
      </div>
      <div class="setting-row">
        <span>Start automatically with the app</span>
        <button
          class="switch"
          :class="{ on: autoStart }"
          data-testid="autostart"
          role="switch"
          :aria-checked="autoStart"
          @click="toggleAutoStart"
        ></button>
      </div>
      <p class="hint">
        Off by default. While running, agents on allowed IPs can drive the vault with the key
        below — including reading secrets once unlocked. Treat the key like a password.
      </p>
      <p v-if="serverError" class="error-text" data-testid="server-error">{{ serverError }}</p>
    </div>

    <div class="panel">
      <h3>Allowed IPs (CIDR)</h3>
      <table class="simple" data-testid="allowlist">
        <tbody>
          <tr v-for="cidr in allowlist" :key="cidr">
            <td class="mono">{{ cidr }}</td>
            <td style="text-align: right">
              <button
                class="btn icon danger"
                :data-testid="'remove-' + cidr"
                @click="removeIP(cidr)"
              >
                ✕
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="row" style="margin-top: 8px">
        <input
          v-model="newIP"
          class="input mono"
          placeholder="e.g. 192.168.0.0/24"
          data-testid="new-ip"
          @keydown.enter="addIP"
        />
        <button class="btn" data-testid="add-ip" @click="addIP">Add</button>
      </div>
      <p v-if="ipError" class="error-text" data-testid="ip-error">{{ ipError }}</p>
    </div>

    <div class="panel">
      <h3>Access key</h3>
      <div class="setting-row">
        <span class="mono" data-testid="api-key">{{ maskedKey() }}</span>
        <span>
          <button class="btn" @click="keyRevealed = !keyRevealed">
            {{ keyRevealed ? "Hide" : "Show" }}
          </button>
          <button class="btn" data-testid="copy-key" @click="copy(apiKey, 'key')">
            {{ copied === "key" ? "Copied!" : "Copy" }}
          </button>
          <button class="btn" data-testid="rotate-key" @click="rotate">Rotate</button>
        </span>
      </div>
      <p class="hint">
        Send it as the <span class="mono">X-API-Key</span> header. Start at
        <span class="mono">GET /v1/ax</span> — it documents the whole API, including
        <span class="mono">/v1/unlock</span>, <span class="mono">/v1/secrets</span> and the
        <span class="mono">/v1/ui/*</span> bridge that operates this very window.
      </p>
    </div>
  </div>
</template>
