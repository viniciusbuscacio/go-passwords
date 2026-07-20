import { reactive } from "vue";
import {
  GetSettings,
  SetTheme,
  SetOpacity,
  IsUnlocked,
  LockVault,
  Version,
  GetAPIStatus,
  GetUpdateInfo,
  CheckForUpdates,
  InstallUpdate,
  SkipUpdateVersion,
  RemindUpdateLater,
  SetUpdateAutoCheck,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { configureToasts } from "./toast";

export type View =
  | "unlock"
  | "vault"
  | "settings"
  | "api"
  | "masterpassword"
  | "categories";

// Minimal shared UI state. A single reactive object is enough of a "router"
// for a small single-window app (family pattern).
export const ui = reactive({
  view: "unlock" as View,
  // Where "back" from Settings/API returns to.
  backTo: "vault" as View,
  theme: "dark",
  opacity: 100,
  unlocked: false,
  version: "",
  // Bumped whenever the vault content changes from outside the current view
  // (REST API writes), so lists can re-fetch.
  refresh: 0,
});

// Live REST server status, mirrored from Go (api:state event). Drives the
// titlebar indicator: a green dot only while a port is actually open.
export const api = reactive({
  running: false,
  port: 0,
  url: "",
});

export function go(view: View) {
  // Settings and the API sub-view can be entered from the titlebar; remember
  // where to land after "back".
  if (view === "settings" || view === "api") {
    if (ui.view === "unlock" || ui.view === "vault") ui.backTo = ui.view;
  }
  ui.view = view;
}

export function back() {
  // Sub-views return to their opener; Settings returns to wherever it came from.
  if (ui.view === "api" || ui.view === "masterpassword") ui.view = "settings";
  else if (ui.view === "categories") ui.view = "vault";
  // If the vault got unlocked while we were in Settings (e.g. via the REST
  // API), "back" must land on the vault, never on a stale unlock screen.
  else ui.view = ui.backTo === "unlock" && ui.unlocked ? "vault" : ui.backTo;
}

export function applyTheme(theme: string) {
  ui.theme = theme;
  document.documentElement.setAttribute("data-theme", theme);
}

export async function toggleTheme() {
  const next = ui.theme === "dark" ? "light" : "dark";
  applyTheme(next);
  await SetTheme(next);
}

export function applyOpacity(percent: number) {
  const clamped = Math.min(100, Math.max(20, percent));
  ui.opacity = clamped;
  document.documentElement.style.setProperty("--bg-alpha", String(clamped / 100));
}

export async function setOpacity(percent: number) {
  applyOpacity(percent);
  await SetOpacity(ui.opacity);
}

export async function lockNow() {
  await LockVault();
  // The vault:locked event flips the view; do it here too for immediacy.
  ui.unlocked = false;
  ui.view = "unlock";
}

export async function loadSettings() {
  const cfg = await GetSettings();
  applyTheme(cfg.theme === "light" ? "light" : "dark");
  applyOpacity(cfg.opacity || 100);
  configureToasts(cfg.toastSeconds ?? 3);
  ui.version = await Version();
  ui.unlocked = await IsUnlocked();
  ui.view = ui.unlocked ? "vault" : "unlock";

  EventsOn("vault:locked", () => {
    ui.unlocked = false;
    ui.view = "unlock";
  });
  EventsOn("vault:unlocked", () => {
    ui.unlocked = true;
    ui.refresh++;
    if (ui.view === "unlock") ui.view = "vault";
  });
  EventsOn("vault:changed", () => {
    ui.refresh++;
  });
  EventsOn("api:state", (s: object) => Object.assign(api, s));
  Object.assign(api, await GetAPIStatus());

  update.autoCheck = cfg.updateAutoCheck;
  Object.assign(update, await GetUpdateInfo());
  EventsOn("update:state", (s: object) => Object.assign(update, s));
}

// ---- In-app updater (family pattern: state lives in Go, UI mirrors it) ----

export const update = reactive({
  checking: false,
  installing: false,
  progress: "",
  available: false,
  version: "",
  notes: "",
  current: "",
  checkedAt: "",
  error: "",
  notify: false,
  // Local additions: the auto-check preference, and whether the user checked
  // manually this session (a manual check shows the result card even for a
  // version that was skipped/snoozed before).
  autoCheck: false,
  seen: false,
});

export async function checkForUpdates() {
  update.seen = true;
  Object.assign(update, await CheckForUpdates());
}

export async function installUpdate() {
  try {
    await InstallUpdate();
  } catch {
    /* the error lands in update.error via the update:state event */
  }
}

export async function skipUpdate() {
  await SkipUpdateVersion();
}

export async function remindUpdateLater() {
  await RemindUpdateLater();
}

export async function setUpdateAutoCheck(v: boolean) {
  update.autoCheck = v;
  await SetUpdateAutoCheck(v);
}
