<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  ExportVault,
  GetSettings,
  ImportVault,
  SetAutoLock,
  SetToastSeconds,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { configureToasts, toast } from "../toast";
import {
  ui,
  back,
  go,
  toggleTheme,
  setOpacity,
  update,
  checkForUpdates,
  installUpdate,
  skipUpdate,
  remindUpdateLater,
  setUpdateAutoCheck,
} from "../store";

const GITHUB_URL = "https://github.com/viniciusbuscacio/go-passwords";

const autoLockEnabled = ref(true);
const autoLockMinutes = ref(5);
const toastSeconds = ref(3);

onMounted(async () => {
  const cfg = await GetSettings();
  autoLockEnabled.value = cfg.autoLockEnabled;
  autoLockMinutes.value = cfg.autoLockMinutes;
  toastSeconds.value = cfg.toastSeconds ?? 3;
});

async function saveToastSeconds() {
  const sec = Math.max(0, Math.min(60, Number(toastSeconds.value) || 0));
  toastSeconds.value = sec;
  await SetToastSeconds(sec);
  configureToasts(sec);
  if (sec > 0) toast.success("Notifications set to " + sec + "s.");
}

function onThemeToggle() {
  toggleTheme();
}

function onOpacity(e: Event) {
  setOpacity(Number((e.target as HTMLInputElement).value));
}

async function onAutoLockToggle(e: Event) {
  autoLockEnabled.value = (e.target as HTMLInputElement).checked;
  await saveAutoLock();
}

async function saveAutoLock() {
  await SetAutoLock(autoLockEnabled.value, Number(autoLockMinutes.value) || 5);
}

async function doExport() {
  try {
    const path = await ExportVault();
    if (path) toast.warning("Exported to " + path + " — cleartext, handle with care.");
  } catch (e) {
    toast.error(String(e));
  }
}

async function doImport() {
  try {
    const n = await ImportVault();
    if (n > 0) {
      toast.success("Imported " + n + " secrets.");
      ui.refresh++;
    }
  } catch (e) {
    toast.error(String(e));
  }
}

function onUpdateAutoCheck(e: Event) {
  setUpdateAutoCheck((e.target as HTMLInputElement).checked);
}

const showUpdateCard = computed(() => update.available && (update.notify || update.seen));

const updateStatus = computed(() => {
  if (update.checking) return "Checking…";
  if (update.installing) return installLabel.value;
  if (update.error) return update.error;
  if (update.available) return `Version ${update.version} is available`;
  if (update.checkedAt) return `You're up to date (${update.current})`;
  return "";
});

const installLabel = computed(() => {
  switch (update.progress) {
    case "downloading":
      return "Downloading…";
    case "verifying":
      return "Verifying…";
    case "applying":
      return "Restarting…";
    default:
      return update.installing ? "Installing…" : "Install and restart";
  }
});
</script>

<template>
  <div class="view view--panel">
    <div class="subheader">
      <button class="icon-btn" title="Back" data-testid="back" @click="back">
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2Z" />
        </svg>
      </button>
      <h1 class="subheader-title">Settings</h1>
    </div>

    <div class="panel-body">
      <p class="section-title">Appearance</p>
      <label class="srow switch-row">
        <span class="row-text">
          <span class="row-label">Dark mode</span>
          <span class="row-desc">Use the dark theme across the app</span>
        </span>
        <input
          type="checkbox"
          class="switch"
          role="switch"
          :checked="ui.theme === 'dark'"
          data-testid="theme-switch"
          @change="onThemeToggle"
        />
      </label>

      <div class="srow">
        <span class="row-text">
          <span class="row-label">Transparency</span>
          <span class="row-desc">Window opacity — {{ ui.opacity }}%</span>
        </span>
        <input
          type="range"
          class="slider"
          min="20"
          max="100"
          step="1"
          :value="ui.opacity"
          data-testid="opacity-slider"
          @input="onOpacity"
        />
      </div>

      <div class="srow">
        <span class="row-text">
          <span class="row-label">Notifications</span>
          <span class="row-desc">Seconds a toast stays on screen — 0 disables them</span>
        </span>
        <input
          v-model="toastSeconds"
          class="input"
          style="width: 80px"
          type="number"
          min="0"
          max="60"
          data-testid="toast-seconds"
          @change="saveToastSeconds"
        />
      </div>

      <p class="section-title">Security</p>
      <label class="srow switch-row">
        <span class="row-text">
          <span class="row-label">Auto-lock</span>
          <span class="row-desc">Lock the vault after a period of inactivity</span>
        </span>
        <input
          type="checkbox"
          class="switch"
          role="switch"
          :checked="autoLockEnabled"
          data-testid="autolock-switch"
          @change="onAutoLockToggle"
        />
      </label>
      <div class="srow">
        <span class="row-text">
          <span class="row-label">Minutes before locking</span>
          <span class="row-desc">Counted from your last action in the app</span>
        </span>
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

      <template v-if="ui.unlocked">
        <p class="section-title">Data</p>
        <div class="srow">
          <span class="row-text">
            <span class="row-label">Export / Import</span>
            <span class="row-desc">Cleartext JSON — handle exports with care</span>
          </span>
          <span style="white-space: nowrap">
            <button class="btn btn-ghost" data-testid="export-vault" @click="doExport">
              Export
            </button>
            <button class="btn btn-ghost" data-testid="import-vault" @click="doImport">
              Import
            </button>
          </span>
        </div>
      </template>

      <p class="section-title">Advanced</p>
      <button class="srow srow--nav" data-testid="nav-api" @click="go('api')">
        <span class="row-text">
          <span class="row-label">REST API Server</span>
          <span class="row-desc">Let agents and automation drive the vault over HTTP</span>
        </span>
        <svg class="chevron" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M10 6 8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6-6-6Z" />
        </svg>
      </button>
      <button
        v-if="ui.unlocked"
        class="srow srow--nav"
        data-testid="nav-master-password"
        @click="go('masterpassword')"
      >
        <span class="row-text">
          <span class="row-label">Change master password</span>
          <span class="row-desc">Re-encrypt the vault key under a new password</span>
        </span>
        <svg class="chevron" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M10 6 8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6-6-6Z" />
        </svg>
      </button>

      <p class="section-title">Updates</p>
      <label class="srow switch-row">
        <span class="row-text">
          <span class="row-label">Check for updates automatically</span>
          <span class="row-desc">Once a day, when the app starts</span>
        </span>
        <input
          type="checkbox"
          class="switch"
          role="switch"
          :checked="update.autoCheck"
          data-testid="update-autocheck"
          @change="onUpdateAutoCheck"
        />
      </label>
      <div class="srow">
        <span class="row-text">
          <span class="row-label">Check for updates</span>
          <span class="row-desc" data-testid="update-status">{{ updateStatus }}</span>
        </span>
        <button
          class="btn"
          data-testid="update-check"
          :disabled="update.checking || update.installing"
          @click="checkForUpdates"
        >
          Check now
        </button>
      </div>

      <div v-if="showUpdateCard" class="update-card">
        <p class="update-card-title">Version {{ update.version }} is available</p>
        <pre v-if="update.notes" class="update-notes" data-testid="update-notes">{{
          update.notes
        }}</pre>
        <div class="update-actions">
          <button
            class="btn"
            data-testid="update-install"
            :disabled="update.installing"
            @click="installUpdate"
          >
            {{ installLabel }}
          </button>
          <button
            class="btn btn-ghost"
            data-testid="update-skip"
            :disabled="update.installing"
            @click="skipUpdate"
          >
            Skip this version
          </button>
          <button
            class="btn btn-ghost"
            data-testid="update-later"
            :disabled="update.installing"
            @click="remindUpdateLater"
          >
            Later
          </button>
        </div>
      </div>

      <p class="section-title">About</p>
      <div class="about">
        <p class="about-desc">
          <strong>go-passwords</strong> is an offline password manager for humans and AI agents,
          built with Go + Wails and TypeScript. The vault is a single fully-encrypted file.
        </p>
        <button class="srow srow--nav" data-testid="open-github" @click="BrowserOpenURL(GITHUB_URL)">
          <span class="row-text">
            <span class="row-label">GitHub</span>
            <span class="row-desc">github.com/viniciusbuscacio/go-passwords</span>
          </span>
          <svg class="chevron" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path
              d="M19 19H5V5h7V3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7h-2v7ZM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7Z"
            />
          </svg>
        </button>
        <p v-if="ui.version" class="about-version" data-testid="app-version">
          Version {{ ui.version }}
        </p>
      </div>
    </div>
  </div>
</template>
