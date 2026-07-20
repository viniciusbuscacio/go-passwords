import { reactive } from "vue";

// Global toast surface (family pattern, adopted from aw's Sonner setup but
// hand-rolled — no dependency). Mounted once in App.vue; call the imperative
// `toast()` / `toast.success()` / `toast.error()` / `toast.warning()` from
// anywhere. Top-right, 4s, hover-revealed close button, constant size,
// slide-in from the right. Variants tint with the app's own tokens.

export type ToastKind = "default" | "success" | "error" | "warning";

export interface ToastItem {
  id: number;
  kind: ToastKind;
  text: string;
}

export const toastState = reactive({ items: [] as ToastItem[] });

// Duration comes from the app settings (Settings → Appearance); 0 disables
// toasts entirely.
let durationMs = 3000;

export function configureToasts(seconds: number) {
  durationMs = Math.max(0, seconds) * 1000;
}

let seq = 0;

function push(kind: ToastKind, text: string) {
  if (durationMs <= 0) return;
  const id = ++seq;
  toastState.items.push({ id, kind, text });
  window.setTimeout(() => dismissToast(id), durationMs);
}

export function dismissToast(id: number) {
  const i = toastState.items.findIndex((t) => t.id === id);
  if (i >= 0) toastState.items.splice(i, 1);
}

export const toast = Object.assign((text: string) => push("default", text), {
  success: (text: string) => push("success", text),
  error: (text: string) => push("error", text),
  warning: (text: string) => push("warning", text),
});
