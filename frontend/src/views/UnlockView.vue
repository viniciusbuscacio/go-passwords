<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import {
  CreateVault,
  LastVault,
  RecentVaults,
  SelectVaultToCreate,
  SelectVaultToOpen,
  UnlockVault,
} from "../../wailsjs/go/main/App";
import { ui } from "../store";

// aw-style gate: a start hub (create / open / recents) that leads to the
// password screen — never a form dumped on the first screen.
const mode = ref<"start" | "unlock" | "create">("start");
const vaultPath = ref("");
const password = ref("");
const password2 = ref("");
const error = ref("");
const busy = ref(false);
const recent = ref<string[]>([]);

onMounted(async () => {
  vaultPath.value = await LastVault();
  recent.value = (await RecentVaults()) ?? [];
});

function baseName(p: string): string {
  const i = Math.max(p.lastIndexOf("\\"), p.lastIndexOf("/"));
  return i >= 0 ? p.slice(i + 1) : p;
}

async function focusPassword() {
  await nextTick();
  (document.querySelector('[data-testid="master-password"]') as HTMLInputElement | null)?.focus();
}

function pickRecent(p: string) {
  vaultPath.value = p;
  error.value = "";
  password.value = "";
  mode.value = "unlock";
  focusPassword();
}

async function openExisting() {
  error.value = "";
  const p = await SelectVaultToOpen();
  if (!p) return;
  vaultPath.value = p;
  password.value = "";
  mode.value = "unlock";
  focusPassword();
}

function startCreate() {
  error.value = "";
  password.value = "";
  password2.value = "";
  mode.value = "create";
}

function backToStart() {
  error.value = "";
  password.value = "";
  password2.value = "";
  mode.value = "start";
}

async function unlock() {
  if (busy.value) return;
  error.value = "";
  busy.value = true;
  try {
    await UnlockVault(vaultPath.value, password.value);
    password.value = "";
    ui.unlocked = true;
    ui.view = "vault";
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}

async function create() {
  if (busy.value) return;
  error.value = "";
  if (!password.value) {
    error.value = "Type a master password.";
    return;
  }
  if (password.value !== password2.value) {
    error.value = "Passwords do not match.";
    return;
  }
  const p = await SelectVaultToCreate();
  if (!p) return;
  busy.value = true;
  try {
    await CreateVault(p, password.value);
    password.value = "";
    password2.value = "";
    ui.unlocked = true;
    ui.view = "vault";
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="view unlock-wrap">
    <div class="unlock-card">
      <!-- Lucide "key-round" -->
      <svg
        class="unlock-glyph"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path
          d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"
        />
        <circle cx="16.5" cy="7.5" r=".5" fill="currentColor" />
      </svg>

      <!-- Start hub: create / open / recents (aw pattern) -->
      <template v-if="mode === 'start'">
        <h2>go-passwords</h2>
        <p class="hint" style="text-align: center; margin-bottom: 14px">
          Create a new encrypted vault or open an existing one.
        </p>

        <button class="action-card primary" data-testid="goto-create" :disabled="busy" @click="startCreate">
          <!-- Lucide "plus-circle" -->
          <svg class="action-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="10" /><path d="M8 12h8" /><path d="M12 8v8" />
          </svg>
          <span class="action-text">
            <span class="action-title">Create a new vault</span>
            <span class="action-desc">Choose where to save the encrypted .gpw file.</span>
          </span>
        </button>

        <button class="action-card" data-testid="browse-vault" :disabled="busy" @click="openExisting">
          <!-- Lucide "folder-open" -->
          <svg class="action-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2" />
          </svg>
          <span class="action-text">
            <span class="action-title">Open an existing vault</span>
            <span class="action-desc">Select a .gpw vault file.</span>
          </span>
        </button>

        <template v-if="recent.length">
          <p class="section-title" style="margin-top: 12px">Recent vaults</p>
          <button
            v-for="(p, i) in recent"
            :key="p"
            class="action-card recent"
            :data-testid="'recent-vault-' + i"
            :disabled="busy"
            @click="pickRecent(p)"
          >
            <!-- Lucide "key-round", small -->
            <svg class="action-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z" />
              <circle cx="16.5" cy="7.5" r=".5" fill="currentColor" />
            </svg>
            <span class="action-text">
              <span class="action-title">{{ baseName(p) }}</span>
              <span class="action-desc">{{ p }}</span>
            </span>
          </button>
        </template>
      </template>

      <!-- Password screen for the chosen vault -->
      <template v-else-if="mode === 'unlock'">
        <h2>Unlock {{ baseName(vaultPath) }}</h2>
        <p class="hint vault-subtitle" data-testid="vault-path" :title="vaultPath">{{ vaultPath }}</p>
        <div class="field">
          <label>Master password</label>
          <input
            v-model="password"
            class="input"
            type="password"
            data-testid="master-password"
            autofocus
            @keydown.enter="unlock"
          />
        </div>
        <button
          class="btn primary"
          style="width: 100%"
          data-testid="unlock-btn"
          :disabled="busy || !vaultPath"
          @click="unlock"
        >
          {{ busy ? "Unlocking…" : "Unlock" }}
        </button>
        <button class="link-btn" data-testid="back-to-start" @click="backToStart">Back</button>
      </template>

      <!-- Create form -->
      <template v-else>
        <h2>Create a new vault</h2>
        <div class="field">
          <label>Master password</label>
          <input v-model="password" class="input" type="password" data-testid="create-password" autofocus />
        </div>
        <div class="field">
          <label>Confirm master password</label>
          <input
            v-model="password2"
            class="input"
            type="password"
            data-testid="create-password2"
            @keydown.enter="create"
          />
        </div>
        <p class="hint" style="margin-bottom: 10px">
          There is no recovery. If you lose this password, the vault is unreadable — by design.
        </p>
        <button class="btn primary" style="width: 100%" data-testid="create-btn" :disabled="busy" @click="create">
          Choose location &amp; create
        </button>
        <button class="link-btn" data-testid="back-to-start" @click="backToStart">Back</button>
      </template>

      <p v-if="error" class="error-text" data-testid="unlock-error">{{ error }}</p>
    </div>
  </div>
</template>
