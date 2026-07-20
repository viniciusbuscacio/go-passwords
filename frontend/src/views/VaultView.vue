<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  CopySecretField,
  DeleteSecret,
  GeneratePassword,
  GetSecret,
  ListCategories,
  ListSecrets,
  SaveSecret,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { vault } from "../../wailsjs/go/models";
import { go, ui } from "../store";
import { toast } from "../toast";

function openURL(url: string) {
  BrowserOpenURL(/^https?:\/\//i.test(url) ? url : "https://" + url);
}

const secrets = ref<vault.Secret[]>([]);
const categories = ref<vault.Category[]>([]);
const query = ref("");
const filterCat = ref("");

const visibleSecrets = computed(() =>
  filterCat.value
    ? secrets.value.filter((s) => s.category_id === filterCat.value)
    : secrets.value,
);

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

function catName(id: string): string {
  return categories.value.find((c) => c.id === id)?.name ?? "";
}

// Chip tint: the category color as a soft background + border, with normal
// foreground text — legible on both themes with any hue.
function chipStyle(id: string) {
  const color = categories.value.find((c) => c.id === id)?.color || "#4cc2ff";
  return {
    background: `color-mix(in srgb, ${color} 18%, transparent)`,
    borderColor: `color-mix(in srgb, ${color} 55%, transparent)`,
  };
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
    toast.error(String(e));
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
    toast.success("Saved.");
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
    toast.success("Deleted.");
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
    toast.success(field === "username" ? "Login copied" : "Password copied");
  } catch (e) {
    toast.error(String(e));
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
        <select v-model="filterCat" class="select cat-filter" data-testid="category-filter">
          <option value="">All categories</option>
          <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <button class="btn primary" data-testid="add-secret" @click="openNew">
          ＋ Add Password
        </button>
        <button class="btn" data-testid="open-categories" @click="go('categories')">
          Categories
        </button>
      </div>

      <div class="secret-list" data-testid="secret-list">
        <div v-if="visibleSecrets.length === 0" class="empty-list">
          No secrets{{ query || filterCat ? " match your filter" : " yet — add your first one" }}.
        </div>
        <div
          v-for="s in visibleSecrets"
          :key="s.id"
          class="secret-row"
          role="button"
          tabindex="0"
          :data-testid="'secret-' + s.id"
          @click="openSecret(s.id)"
          @keydown.enter="openSecret(s.id)"
        >
          <span class="title">{{ s.title }}</span>
          <span v-if="s.username" class="sub">{{ s.username }}</span>
          <span v-if="s.category_id" class="chip" :style="chipStyle(s.category_id)">{{
            catName(s.category_id)
          }}</span>
          <span class="row-actions">
            <button
              class="icon-btn small"
              data-tip="Copy Username"
              :data-testid="'copy-user-' + s.id"
              @click.stop="copyField(s.id, 'username')"
            >
              <!-- Lucide "user" -->
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
            </button>
            <button
              class="icon-btn small"
              data-tip="Copy Password"
              :data-testid="'copy-pass-' + s.id"
              @click.stop="copyField(s.id, 'password')"
            >
              <!-- Lucide "key-round" -->
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z" />
                <circle cx="16.5" cy="7.5" r=".5" fill="currentColor" />
              </svg>
            </button>
            <button
              v-if="s.url"
              class="icon-btn small"
              data-tip="Open URL"
              :data-testid="'open-url-' + s.id"
              @click.stop="openURL(s.url!)"
            >
              <!-- Lucide "external-link" -->
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M15 3h6v6" />
                <path d="M10 14 21 3" />
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              </svg>
            </button>
          </span>
        </div>
      </div>
    </template>

    <!-- Full-window editor view. NEVER a side drawer (family rule, 18/jul/2026):
         the form owns the whole window, with a back button. -->
    <div v-else class="editor-page">
      <div class="subheader" style="padding-left: 0">
        <button
          class="icon-btn"
          data-testid="close-editor"
          title="Back to the list"
          @click="editor.open = false"
        >
          <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2Z" />
          </svg>
        </button>
        <h1 class="subheader-title">{{ editor.id ? "Edit secret" : "New secret" }}</h1>
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
              :title="editor.showPassword ? 'Hide password' : 'Show password'"
              data-testid="toggle-password"
              @click="editor.showPassword = !editor.showPassword"
            >
              <!-- Fluent System Icons (MIT): eye / eye-off — the Windows 11
                   password-reveal glyphs. -->
              <svg
                v-if="!editor.showPassword"
                class="ctrl-icon"
                viewBox="0 0 24 24"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  d="M11.9999 9.00462C14.209 9.00462 15.9999 10.7955 15.9999 13.0046C15.9999 15.2138 14.209 17.0046 11.9999 17.0046C9.79073 17.0046 7.99987 15.2138 7.99987 13.0046C7.99987 10.7955 9.79073 9.00462 11.9999 9.00462ZM11.9999 10.5046C10.6192 10.5046 9.49987 11.6239 9.49987 13.0046C9.49987 14.3853 10.6192 15.5046 11.9999 15.5046C13.3806 15.5046 14.4999 14.3853 14.4999 13.0046C14.4999 11.6239 13.3806 10.5046 11.9999 10.5046ZM11.9999 5.5C16.6134 5.5 20.596 8.65001 21.701 13.0644C21.8016 13.4662 21.5574 13.8735 21.1556 13.9741C20.7537 14.0746 20.3465 13.8305 20.2459 13.4286C19.307 9.67796 15.9212 7 11.9999 7C8.07681 7 4.68997 9.68026 3.75273 13.4332C3.65237 13.835 3.24523 14.0794 2.84336 13.9791C2.44149 13.8787 2.19707 13.4716 2.29743 13.0697C3.40052 8.65272 7.38436 5.5 11.9999 5.5Z"
                />
              </svg>
              <svg
                v-else
                class="ctrl-icon"
                viewBox="0 0 24 24"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  d="M2.21967 2.21967C1.9534 2.48594 1.9292 2.9026 2.14705 3.19621L2.21967 3.28033L6.25424 7.3149C4.33225 8.66437 2.89577 10.6799 2.29888 13.0644C2.1983 13.4662 2.4425 13.8735 2.84431 13.9741C3.24613 14.0746 3.6534 13.8305 3.75399 13.4286C4.28346 11.3135 5.59112 9.53947 7.33416 8.39452L9.14379 10.2043C8.43628 10.9258 8 11.9143 8 13.0046C8 15.2138 9.79086 17.0046 12 17.0046C13.0904 17.0046 14.0788 16.5683 14.8004 15.8608L20.7197 21.7803C21.0126 22.0732 21.4874 22.0732 21.7803 21.7803C22.0466 21.5141 22.0708 21.0974 21.8529 20.8038L21.7803 20.7197L15.6668 14.6055L15.668 14.604L14.4679 13.4061L11.598 10.5368L11.6 10.536L8.71877 7.65782L8.72 7.656L7.58672 6.52549L3.28033 2.21967C2.98744 1.92678 2.51256 1.92678 2.21967 2.21967ZM10.2041 11.2655L13.7392 14.8006C13.2892 15.2364 12.6759 15.5046 12 15.5046C10.6193 15.5046 9.5 14.3853 9.5 13.0046C9.5 12.3287 9.76824 11.7154 10.2041 11.2655ZM12 5.5C10.9997 5.5 10.0291 5.64807 9.11109 5.925L10.3481 7.16119C10.8839 7.05532 11.4364 7 12 7C15.9231 7 19.3099 9.68026 20.2471 13.4332C20.3475 13.835 20.7546 14.0794 21.1565 13.9791C21.5584 13.8787 21.8028 13.4716 21.7024 13.0697C20.5994 8.65272 16.6155 5.5 12 5.5ZM12.1947 9.00928L15.996 12.81C15.8942 10.7531 14.2472 9.10764 12.1947 9.00928Z"
                />
              </svg>
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
        <button class="btn" data-testid="cancel-editor" @click="editor.open = false">Cancel</button>
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

  </div>
</template>
