import { reactive } from "vue";
import {
  GetSettings,
  SetTheme,
  IsUnlocked,
  LockVault,
  Version,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

export type View = "unlock" | "vault" | "settings" | "api";

// Minimal shared UI state. A single reactive object is enough of a "router"
// for a small single-window app (family pattern).
export const ui = reactive({
  view: "unlock" as View,
  // Where "back" from Settings/API returns to.
  backTo: "vault" as View,
  theme: "dark",
  unlocked: false,
  version: "",
  // Bumped whenever the vault content changes from outside the current view
  // (REST API writes), so lists can re-fetch.
  refresh: 0,
});

export function go(view: View) {
  if (view === "settings" || view === "api") {
    if (ui.view === "unlock" || ui.view === "vault") ui.backTo = ui.view;
  }
  ui.view = view;
}

export function back() {
  ui.view = ui.view === "api" ? "settings" : ui.backTo;
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

export async function lockNow() {
  await LockVault();
  // The vault:locked event flips the view; do it here too for immediacy.
  ui.unlocked = false;
  ui.view = "unlock";
}

export async function loadSettings() {
  const cfg = await GetSettings();
  applyTheme(cfg.theme === "light" ? "light" : "dark");
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
}
