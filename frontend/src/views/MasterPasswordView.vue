<script setup lang="ts">
import { ref } from "vue";
import { ChangeMasterPassword } from "../../wailsjs/go/main/App";
import { back } from "../store";
import { toast } from "../toast";

const current = ref("");
const newPw = ref("");
const confirm = ref("");
const msg = ref("");
const err = ref("");

async function change() {
  err.value = "";
  msg.value = "";
  if (!newPw.value) {
    err.value = "Type the new password.";
    return;
  }
  if (newPw.value !== confirm.value) {
    err.value = "New passwords do not match.";
    return;
  }
  try {
    await ChangeMasterPassword(current.value, newPw.value);
    current.value = newPw.value = confirm.value = "";
    toast.success("Master password changed.");
  } catch (e) {
    err.value = String(e);
  }
}
</script>

<template>
  <div class="view view--panel">
    <div class="subheader">
      <button class="icon-btn" title="Back" data-testid="back" @click="back">
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2Z" />
        </svg>
      </button>
      <h1 class="subheader-title">Change master password</h1>
    </div>

    <div class="panel-body">
      <p class="about-desc">
        The vault key is re-encrypted under the new password — your data is untouched. There is
        no recovery: if you lose the new password, the vault is unreadable, by design.
      </p>
      <div class="srow srow--form">
        <span class="row-desc">Current password</span>
        <input v-model="current" class="input" type="password" data-testid="mp-current" autofocus />
        <span class="row-desc">New password</span>
        <input v-model="newPw" class="input" type="password" data-testid="mp-new" />
        <span class="row-desc">Confirm new password</span>
        <input
          v-model="confirm"
          class="input"
          type="password"
          data-testid="mp-confirm"
          @keydown.enter="change"
        />
        <div>
          <button class="btn primary" data-testid="mp-change" @click="change">
            Change master password
          </button>
        </div>
        <p v-if="err" class="error-text">{{ err }}</p>
        <p v-if="msg" class="row-desc">{{ msg }}</p>
      </div>
    </div>
  </div>
</template>
