<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue";
import {
  CopySecretField,
  DeleteSecret,
  GeneratePassword,
  GetSecret,
  ListCategories,
  ListSecrets,
  SaveSecret,
} from "../../wailsjs/go/main/App";
import { vault } from "../../wailsjs/go/models";
import { ui } from "../store";

const secrets = ref<vault.Secret[]>([]);
const categories = ref<vault.Category[]>([]);
const query = ref("");
const toast = ref("");
let toastTimer: ReturnType<typeof setTimeout> | undefined;

const editor = reactive({
  open: false,
  id: "",
  title: "",
  username: "",
  password: "",
  url: "",
  notes: "",
  categoryId: "",
  showPassword: false,
  confirmingDelete: false,
  error: "",
});

async function refresh() {
  try {
    secrets.value = await ListSecrets(query.value);
    categories.value = await ListCategories();
  } catch {
    // Vault got locked under us; the vault:locked event flips the view.
  }
}

onMounted(refresh);
watch(query, refresh);
// REST API writes bump ui.refresh so the list stays honest.
watch(() => ui.refresh, refresh);

function showToast(msg: string) {
  toast.value = msg;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (toast.value = ""), 1800);
}

function catName(id: string): string {
  return categories.value.find((c) => c.id === id)?.name ?? "";
}

function openNew() {
  Object.assign(editor, {
    open: true,
    id: "",
    title: "",
    username: "",
    password: "",
    url: "",
    notes: "",
    categoryId: "",
    showPassword: false,
    confirmingDelete: false,
    error: "",
  });
}

async function openSecret(id: string) {
  try {
    const s = await GetSecret(id);
    Object.assign(editor, {
      open: true,
      id: s.id,
      title: s.title,
      username: s.username ?? "",
      password: s.password ?? "",
      url: s.url ?? "",
      notes: s.notes ?? "",
      categoryId: s.category_id ?? "",
      showPassword: false,
      confirmingDelete: false,
      error: "",
    });
  } catch (e) {
    showToast(String(e));
  }
}

async function save() {
  editor.error = "";
  try {
    await SaveSecret(editor.id, {
      Title: editor.title,
      Username: editor.username,
      Password: editor.password,
      URL: editor.url,
      Notes: editor.notes,
      CategoryID: editor.categoryId,
    });
    editor.open = false;
    await refresh();
    showToast("Saved.");
  } catch (e) {
    editor.error = String(e);
  }
}

async function del() {
  if (!editor.confirmingDelete) {
    editor.confirmingDelete = true;
    return;
  }
  try {
    await DeleteSecret(editor.id);
    editor.open = false;
    await refresh();
    showToast("Deleted.");
  } catch (e) {
    editor.error = String(e);
  }
}

async function generate() {
  try {
    editor.password = await GeneratePassword(16, true);
    editor.showPassword = true;
  } catch (e) {
    editor.error = String(e);
  }
}

async function copyField(id: string, field: string) {
  try {
    await CopySecretField(id, field);
    showToast(field + " copied");
  } catch (e) {
    showToast(String(e));
  }
}
</script>

<template>
  <div class="view">
    <template v-if="!editor.open">
      <div class="toolbar">
        <input
          v-model="query"
          class="input"
          type="text"
          placeholder="Search…"
          data-testid="search"
        />
        <button class="btn primary" data-testid="add-secret" @click="openNew">＋ Add</button>
      </div>

      <div class="secret-list" data-testid="secret-list">
        <div v-if="secrets.length === 0" class="empty-list">
          No secrets{{ query ? " match your search" : " yet — add your first one" }}.
        </div>
        <button
          v-for="s in secrets"
          :key="s.id"
          class="secret-row"
          :data-testid="'secret-' + s.id"
          @click="openSecret(s.id)"
        >
          <span class="title">{{ s.title }}</span>
          <span v-if="s.username" class="sub">{{ s.username }}</span>
          <span v-if="s.category_id" class="chip">{{ catName(s.category_id) }}</span>
        </button>
      </div>
    </template>

    <!-- Full-window editor view. NEVER a side drawer (family rule, 18/jul/2026):
         the form owns the whole window, with a back button. -->
    <div v-else class="editor-page">
      <div class="view-title">
        <button
          class="btn icon"
          data-testid="close-editor"
          title="Back to the list"
          @click="editor.open = false"
        >
          ←
        </button>
        {{ editor.id ? "Edit secret" : "New secret" }}
      </div>

      <div class="field">
          <label>Title *</label>
          <input v-model="editor.title" class="input" data-testid="edit-title" autofocus />
        </div>
        <div class="field">
          <label>Username</label>
          <div class="row">
            <input v-model="editor.username" class="input" data-testid="edit-username" />
            <button
              v-if="editor.id"
              class="btn icon"
              title="Copy username"
              data-testid="copy-username"
              @click="copyField(editor.id, 'username')"
            >
              ⧉
            </button>
          </div>
        </div>
        <div class="field">
          <label>Password</label>
          <div class="row">
            <input
              v-model="editor.password"
              class="input mono"
              :type="editor.showPassword ? 'text' : 'password'"
              data-testid="edit-password"
            />
            <button
              class="btn icon"
              :title="editor.showPassword ? 'Hide' : 'Show'"
              data-testid="toggle-password"
              @click="editor.showPassword = !editor.showPassword"
            >
              {{ editor.showPassword ? "🙈" : "👁" }}
            </button>
            <button
              v-if="editor.id"
              class="btn icon"
              title="Copy password"
              data-testid="copy-password"
              @click="copyField(editor.id, 'password')"
            >
              ⧉
            </button>
          </div>
          <button class="btn" data-testid="generate-password" @click="generate">
            Generate strong password
          </button>
        </div>
        <div class="field">
          <label>URL</label>
          <input v-model="editor.url" class="input" data-testid="edit-url" />
        </div>
        <div class="field">
          <label>Notes</label>
          <textarea v-model="editor.notes" class="textarea" data-testid="edit-notes"></textarea>
        </div>
        <div class="field">
          <label>Category</label>
          <select v-model="editor.categoryId" class="select" data-testid="edit-category">
            <option value="">— none —</option>
            <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>

      <p v-if="editor.error" class="error-text" data-testid="editor-error">
        {{ editor.error }}
      </p>

      <div class="row" style="padding-top: 14px">
        <button class="btn primary" data-testid="save-secret" @click="save">Save</button>
        <button
          v-if="editor.id && !editor.confirmingDelete"
          class="btn danger"
          data-testid="delete-secret"
          @click="del"
        >
          Delete
        </button>
        <button
          v-if="editor.id && editor.confirmingDelete"
          class="btn danger"
          data-testid="confirm-delete"
          @click="del"
        >
          Really delete?
        </button>
      </div>
    </div>

    <div v-if="toast" class="toast">{{ toast }}</div>
  </div>
</template>
