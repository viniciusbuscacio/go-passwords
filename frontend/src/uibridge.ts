import { nextTick } from "vue";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { UIAck } from "../wailsjs/go/main/App";
import { ui } from "./store";

// The Go REST server sends a "ui:command"; we perform it against the REAL DOM
// (click the button, press the key, type), let Vue re-render, then report the
// resulting on-screen state back via UIAck. This is what lets an external
// agent actually operate the UI (family pattern, from go-calc).

interface UICommand {
  id: string;
  type: "state" | "press" | "dblclick" | "key" | "input";
  testid?: string;
  key?: string;
  value?: string;
}

function el(testid: string): HTMLElement | null {
  return document.querySelector(`[data-testid="${testid}"]`);
}

function text(testid: string): string | undefined {
  const node = el(testid);
  return node ? (node.textContent ?? "").trim() : undefined;
}

// A snapshot of what is currently on screen, read from the rendered DOM.
// Passwords are NEVER included: masked inputs report only their length.
function collectState() {
  const controls = Array.from(document.querySelectorAll("[data-testid]"))
    .map((e) => e.getAttribute("data-testid"))
    .filter((v): v is string => !!v);

  const state: Record<string, unknown> = {
    view: ui.view,
    theme: ui.theme,
    unlocked: ui.unlocked,
    controls,
  };

  for (const t of ["vault-path", "unlock-error", "editor-error", "status", "app-version"]) {
    const v = text(t);
    if (v !== undefined) state[t.replace(/-/g, "_")] = v;
  }

  const list = el("secret-list");
  if (list) {
    state.secrets = Array.from(list.querySelectorAll(".secret-row .title")).map((n) =>
      (n.textContent ?? "").trim(),
    );
  }
  const allow = el("allowlist");
  if (allow) {
    state.allowlist = Array.from(allow.querySelectorAll("td.mono")).map((td) =>
      (td.textContent ?? "").trim(),
    );
  }

  const inputs: Record<string, string> = {};
  document
    .querySelectorAll("input[data-testid], textarea[data-testid], select[data-testid]")
    .forEach((n) => {
      const t = n.getAttribute("data-testid");
      if (!t) return;
      const input = n as HTMLInputElement;
      // Never report a password's content, only that it has one.
      inputs[t] = input.type === "password" ? `(${input.value.length} chars)` : input.value;
    });
  if (Object.keys(inputs).length) state.inputs = inputs;

  return state;
}

interface UIError {
  code: "unknown_testid" | "disabled_control";
  message: string;
}

function isDisabled(node: HTMLElement): boolean {
  return (
    node.hasAttribute("disabled") ||
    (node as HTMLButtonElement).disabled === true ||
    node.getAttribute("aria-disabled") === "true"
  );
}

// settle waits until the DOM stops changing: async work (unlock runs Argon2id
// in Go, list refetches…) finishes at its own pace, so we poll until two
// consecutive snapshots are identical, bounded so nothing can hang the bridge.
async function settle() {
  await nextTick();
  const deadline = performance.now() + 3000;
  let prev = JSON.stringify(collectState());
  while (performance.now() < deadline) {
    await new Promise((r) => window.setTimeout(r, 80));
    await nextTick();
    const cur = JSON.stringify(collectState());
    if (cur === prev) return;
    prev = cur;
  }
}

// setNativeValue drives a form control the way v-model expects: native setter
// + input/change events. Works for input, textarea and select alike.
function setNativeValue(node: HTMLElement, value: string) {
  const proto =
    node instanceof HTMLTextAreaElement
      ? HTMLTextAreaElement.prototype
      : node instanceof HTMLSelectElement
        ? HTMLSelectElement.prototype
        : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  setter ? setter.call(node, value) : ((node as HTMLInputElement).value = value);
  node.dispatchEvent(new Event("input", { bubbles: true }));
  node.dispatchEvent(new Event("change", { bubbles: true }));
}

async function perform(cmd: UICommand): Promise<UIError | undefined> {
  let error: UIError | undefined;

  if (cmd.type === "press" || cmd.type === "dblclick") {
    const node = cmd.testid ? el(cmd.testid) : null;
    if (!node) {
      error = { code: "unknown_testid", message: `unknown testid: ${cmd.testid}` };
    } else if (isDisabled(node)) {
      error = { code: "disabled_control", message: `control is disabled: ${cmd.testid}` };
    } else if (cmd.type === "press") {
      node.click();
    } else {
      for (const detail of [1, 2]) {
        node.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, detail }));
        node.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, detail }));
        node.dispatchEvent(new MouseEvent("click", { bubbles: true, detail }));
      }
      node.dispatchEvent(new MouseEvent("dblclick", { bubbles: true, detail: 2 }));
    }
  } else if (cmd.type === "key" && cmd.key) {
    const target = document.activeElement ?? window;
    target.dispatchEvent(new KeyboardEvent("keydown", { key: cmd.key, bubbles: true }));
  } else if (cmd.type === "input") {
    const node = cmd.testid ? el(cmd.testid) : null;
    if (!node) {
      error = { code: "unknown_testid", message: `unknown testid: ${cmd.testid}` };
    } else if (isDisabled(node)) {
      error = { code: "disabled_control", message: `control is disabled: ${cmd.testid}` };
    } else {
      setNativeValue(node, cmd.value ?? "");
    }
  }

  await settle();
  return error;
}

// Commands are serialized so two DOM mutations never overlap.
let queue: Promise<void> = Promise.resolve();

async function run(cmd: UICommand) {
  let error: UIError | undefined;
  try {
    error = await perform(cmd);
  } catch {
    /* still report whatever is on screen */
  }
  const state = collectState();
  if (error) state.error = error;
  try {
    await UIAck(cmd.id, JSON.stringify(state));
  } catch {
    /* the Go side will time out */
  }
}

export function initUIBridge() {
  EventsOn("ui:command", (cmd: UICommand) => {
    queue = queue.then(() => run(cmd));
  });
}
