<script setup lang="ts">
import { onMounted, ref } from "vue";
import {
  CreateVault,
  LastVault,
  SelectVaultToCreate,
  SelectVaultToOpen,
  UnlockVault,
} from "../../wailsjs/go/main/App";
import { ui } from "../store";

const mode = ref<"open" | "create">("open");
const vaultPath = ref("");
const password = ref("");
const password2 = ref("");
const error = ref("");
const busy = ref(false);

onMounted(async () => {
  vaultPath.value = await LastVault();
});

async function browse() {
  error.value = "";
  const p = await SelectVaultToOpen();
  if (p) vaultPath.value = p;
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

      <template v-if="mode === 'open'">
        <h2>Unlock your vault</h2>
        <div class="field">
          <label>Vault file</label>
          <div class="row">
            <span class="vault-path" data-testid="vault-path">{{
              vaultPath || "No vault selected"
            }}</span>
            <button class="btn" data-testid="browse-vault" @click="browse">Browse…</button>
          </div>
        </div>
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
        <button class="link-btn" data-testid="goto-create" @click="mode = 'create'; error = ''">
          Create a new vault instead
        </button>
      </template>

      <template v-else>
        <h2>Create a new vault</h2>
        <div class="field">
          <label>Master password</label>
          <input
            v-model="password"
            class="input"
            type="password"
            data-testid="create-password"
            autofocus
          />
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
        <button
          class="btn primary"
          style="width: 100%"
          data-testid="create-btn"
          :disabled="busy"
          @click="create"
        >
          Choose location &amp; create
        </button>
        <button class="link-btn" data-testid="goto-open" @click="mode = 'open'; error = ''">
          Open an existing vault
        </button>
      </template>

      <p v-if="error" class="error-text" data-testid="unlock-error">{{ error }}</p>
    </div>
  </div>
</template>
