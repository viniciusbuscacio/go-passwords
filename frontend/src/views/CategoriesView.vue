<script setup lang="ts">
import { onMounted, ref } from "vue";
import {
  AddCategory,
  CategoryPalette,
  DeleteCategory,
  ListCategories,
  RenameCategory,
  SetCategoryColor,
} from "../../wailsjs/go/main/App";
import { vault } from "../../wailsjs/go/models";
import { back, ui } from "../store";
import { toast } from "../toast";

// Draft-based editing (family rule): every change — name, color, add, delete —
// lives only in this local draft until Save. Cancel/back discards everything.
interface DraftCat {
  id: string;
  name: string;
  color: string;
  isNew?: boolean;
}

const original = ref<vault.Category[]>([]);
const draft = ref<DraftCat[]>([]);
const palette = ref<string[]>([]);
const newCategory = ref("");
const busy = ref(false);
let newSeq = 0;

onMounted(async () => {
  palette.value = await CategoryPalette();
  await load();
});

async function load() {
  try {
    original.value = await ListCategories();
  } catch {
    original.value = [];
  }
  draft.value = original.value.map((c) => ({
    id: c.id,
    name: c.name,
    color: c.color || "",
  }));
}

function add() {
  const name = newCategory.value.trim();
  if (!name) return;
  draft.value.push({
    id: "new-" + ++newSeq,
    name,
    color: palette.value[draft.value.length % palette.value.length] || "",
    isNew: true,
  });
  newCategory.value = "";
}

function remove(id: string) {
  draft.value = draft.value.filter((d) => d.id !== id);
}

async function save() {
  if (busy.value) return;
  if (draft.value.some((d) => !d.name.trim())) {
    toast.error("Category names cannot be empty.");
    return;
  }
  busy.value = true;
  try {
    // Deletes: originals no longer present in the draft.
    for (const o of original.value) {
      if (!draft.value.some((d) => d.id === o.id)) await DeleteCategory(o.id);
    }
    // Adds and edits.
    for (const d of draft.value) {
      if (d.isNew) {
        const c = await AddCategory(d.name.trim());
        if (d.color) await SetCategoryColor(c.id, d.color);
      } else {
        const o = original.value.find((x) => x.id === d.id);
        if (!o) continue;
        if (o.name !== d.name.trim()) await RenameCategory(d.id, d.name.trim());
        if ((o.color || "") !== d.color) await SetCategoryColor(d.id, d.color);
      }
    }
    ui.refresh++;
    toast.success("Categories saved.");
    back();
  } catch (e) {
    toast.error(String(e));
    await load();
  } finally {
    busy.value = false;
  }
}

function cancel() {
  back();
}
</script>

<template>
  <div class="view view--panel">
    <div class="subheader">
      <button class="icon-btn" title="Back (discards changes)" data-testid="back" @click="cancel">
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2Z" />
        </svg>
      </button>
      <h1 class="subheader-title">Categories</h1>
    </div>

    <div class="panel-body">
      <div class="srow">
        <input
          v-model="newCategory"
          class="input"
          placeholder="New category…"
          data-testid="new-category"
          autofocus
          @keydown.enter="add"
        />
        <button class="btn" data-testid="add-category" @click="add">Add</button>
      </div>

      <p v-if="draft.length" class="section-title">Your categories</p>
      <div v-for="d in draft" :key="d.id" class="srow" data-testid="category-list">
        <span class="cat-name" style="flex: 1; min-width: 0">
          <span class="cat-dot" :style="{ background: d.color || '#4cc2ff' }"></span>
          <input v-model="d.name" class="input" :data-testid="'cat-name-' + d.id" />
        </span>
        <span class="cat-actions">
          <span class="swatches">
            <button
              v-for="(col, n) in palette"
              :key="col"
              class="swatch"
              :class="{ active: d.color === col }"
              :style="{ background: col }"
              :data-testid="'cat-color-' + d.id + '-' + n"
              @click="d.color = col"
            ></button>
          </span>
          <button class="btn btn-ghost danger" :data-testid="'cat-delete-' + d.id" @click="remove(d.id)">
            Delete
          </button>
        </span>
      </div>
      <p v-if="!draft.length" class="row-desc" style="padding: 0 2px">
        No categories yet. They group your passwords in the list.
      </p>

      <div class="row" style="padding-top: 8px">
        <button class="btn primary" data-testid="save-categories" :disabled="busy" @click="save">
          Save
        </button>
        <button class="btn" data-testid="cancel-categories" :disabled="busy" @click="cancel">
          Cancel
        </button>
      </div>
    </div>
  </div>
</template>
