<script setup lang="ts">
import { onMounted, ref } from "vue";
import {
  AddCategory,
  ChangeMasterPassword,
  DeleteCategory,
  ExportVault,
  GetSettings,
  ImportVault,
  ListCategories,
  RenameCategory,
  SetAutoLock,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { vault } from "../../wailsjs/go/models";
import { ui, back, go, toggleTheme } from "../store";

const autoLockEnabled = ref(true);
const autoLockMinutes = ref(5);
const categories = ref<vault.Category[]>([]);
const newCategory = ref("");
const mpCurrent = ref("");
const mpNew = ref("");
const mpConfirm = ref("");
const mpMsg = ref("");
const mpErr = ref("");
const dataMsg = ref("");

onMounted(async () => {
  const cfg = await GetSettings();
  autoLockEnabled.value = cfg.autoLockEnabled;
  autoLockMinutes.value = cfg.autoLockMinutes;
  if (ui.unlocked) await refreshCategories();
});

async function refreshCategories() {
  try {
    categories.value = await ListCategories();
  } catch {
    categories.value = [];
  }
}

async function saveAutoLock() {
  await SetAutoLock(autoLockEnabled.value, Number(autoLockMinutes.value) || 5);
}

async function toggleAutoLock() {
  autoLockEnabled.value = !autoLockEnabled.value;
  await saveAutoLock();
}

async function addCat() {
  if (!newCategory.value.trim()) return;
  await AddCategory(newCategory.value.trim());
  newCategory.value = "";
  await refreshCategories();
}

async function renameCat(c: vault.Category) {
  const name = prompt("New name for category:", c.name);
  if (name && name.trim() && name !== c.name) {
    await RenameCategory(c.id, name.trim());
    await refreshCategories();
  }
}

async function deleteCat(id: string) {
  await DeleteCategory(id);
  await refreshCategories();
}

async function changeMP() {
  mpErr.value = "";
  mpMsg.value = "";
  if (!mpNew.value) {
    mpErr.value = "Type the new password.";
    return;
  }
  if (mpNew.value !== mpConfirm.value) {
    mpErr.value = "New passwords do not match.";
    return;
  }
  try {
    await ChangeMasterPassword(mpCurrent.value, mpNew.value);
    mpCurrent.value = mpNew.value = mpConfirm.value = "";
    mpMsg.value = "Master password changed.";
  } catch (e) {
    mpErr.value = String(e);
  }
}

async function doExport() {
  dataMsg.value = "";
  try {
    const path = await ExportVault();
    if (path) dataMsg.value = "Exported to " + path + " — this file is CLEARTEXT, handle with care.";
  } catch (e) {
    dataMsg.value = String(e);
  }
}

async function doImport() {
  dataMsg.value = "";
  try {
    const n = await ImportVault();
    if (n > 0) {
      dataMsg.value = "Imported " + n + " secrets.";
      ui.refresh++;
    }
  } catch (e) {
    dataMsg.value = String(e);
  }
}
</script>

<template>
  <div class="view">
    <div class="view-title">
      <button class="btn icon" data-testid="back" title="Back" @click="back">←</button>
      Settings
    </div>

    <div class="panel">
      <h3>Appearance</h3>
      <div class="setting-row">
        <span>Dark mode</span>
        <button
          class="switch"
          :class="{ on: ui.theme === 'dark' }"
          data-testid="theme-switch"
          role="switch"
          :aria-checked="ui.theme === 'dark'"
          @click="toggleTheme"
        ></button>
      </div>
    </div>

    <div class="panel">
      <h3>Security</h3>
      <div class="setting-row">
        <span>Auto-lock after inactivity</span>
        <button
          class="switch"
          :class="{ on: autoLockEnabled }"
          data-testid="autolock-switch"
          role="switch"
          :aria-checked="autoLockEnabled"
          @click="toggleAutoLock"
        ></button>
      </div>
      <div class="setting-row">
        <span>Minutes before locking</span>
        <input
          v-model="autoLockMinutes"
          class="input"
          style="width: 80px"
          type="number"
          min="1"
          max="720"
          data-testid="autolock-minutes"
          :disabled="!autoLockEnabled"
          @change="saveAutoLock"
        />
      </div>
    </div>

    <div v-if="ui.unlocked" class="panel">
      <h3>Categories</h3>
      <table v-if="categories.length" class="simple" data-testid="category-list">
        <tbody>
          <tr v-for="c in categories" :key="c.id">
            <td>{{ c.name }}</td>
            <td style="text-align: right; white-space: nowrap">
              <button class="btn icon" :data-testid="'cat-rename-' + c.id" @click="renameCat(c)">
                ✎
              </button>
              <button
                class="btn icon danger"
                :data-testid="'cat-delete-' + c.id"
                @click="deleteCat(c.id)"
              >
                ✕
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="row" style="margin-top: 8px">
        <input
          v-model="newCategory"
          class="input"
          placeholder="New category…"
          data-testid="new-category"
          @keydown.enter="addCat"
        />
        <button class="btn" data-testid="add-category" @click="addCat">Add</button>
      </div>
    </div>

    <div v-if="ui.unlocked" class="panel">
      <h3>Master password</h3>
      <div class="field">
        <label>Current password</label>
        <input v-model="mpCurrent" class="input" type="password" data-testid="mp-current" />
      </div>
      <div class="field">
        <label>New password</label>
        <input v-model="mpNew" class="input" type="password" data-testid="mp-new" />
      </div>
      <div class="field">
        <label>Confirm new password</label>
        <input v-model="mpConfirm" class="input" type="password" data-testid="mp-confirm" />
      </div>
      <button class="btn primary" data-testid="mp-change" @click="changeMP">
        Change master password
      </button>
      <p v-if="mpErr" class="error-text">{{ mpErr }}</p>
      <p v-if="mpMsg" class="hint" style="margin-top: 6px">{{ mpMsg }}</p>
    </div>

    <div v-if="ui.unlocked" class="panel">
      <h3>Data</h3>
      <div class="row">
        <button class="btn" data-testid="export-vault" @click="doExport">Export (cleartext JSON)</button>
        <button class="btn" data-testid="import-vault" @click="doImport">Import</button>
      </div>
      <p v-if="dataMsg" class="hint" style="margin-top: 8px">{{ dataMsg }}</p>
    </div>

    <div class="panel">
      <h3>Agent access</h3>
      <div class="setting-row">
        <span>REST API server for agents and automation</span>
        <button class="btn" data-testid="nav-api" @click="go('api')">Open</button>
      </div>
    </div>

    <div class="panel">
      <h3>About</h3>
      <div class="setting-row">
        <span>
          go-passwords <span class="mono" data-testid="app-version">{{ ui.version }}</span>
        </span>
        <button
          class="btn"
          data-testid="open-github"
          @click="BrowserOpenURL('https://github.com/viniciusbuscacio/go-passwords')"
        >
          GitHub
        </button>
      </div>
    </div>
  </div>
</template>
