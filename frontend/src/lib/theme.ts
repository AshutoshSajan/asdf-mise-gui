export type Theme = "system" | "light" | "dark";

const STORAGE_KEY = "asdf-mise-gui.theme";
const LEGACY_STORAGE_KEY = "asdf-gui.theme";
const THEMES: Theme[] = ["system", "light", "dark"];

let systemQuery: MediaQueryList | null = null;
let systemListener: ((event: MediaQueryListEvent) => void) | null = null;

export function normalizeTheme(value: unknown): Theme {
  return typeof value === "string" && (THEMES as string[]).includes(value) ? (value as Theme) : "dark";
}

export function readStoredTheme(): Theme {
  try {
    // Fall back to the pre-rename key so upgrades keep the saved theme.
    return normalizeTheme(
      window.localStorage.getItem(STORAGE_KEY) ?? window.localStorage.getItem(LEGACY_STORAGE_KEY),
    );
  } catch {
    return "dark";
  }
}

export function resolvedTheme(theme: Theme): "light" | "dark" {
  if (theme === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return theme;
}

function detachSystemListener() {
  if (systemQuery && systemListener) {
    systemQuery.removeEventListener("change", systemListener);
  }
  systemQuery = null;
  systemListener = null;
}

// applyTheme is the only place that touches the root theme classes. It also
// mirrors the choice to localStorage so the first paint after a restart uses
// the last saved theme instead of the dark default in index.html.
export function applyTheme(value: unknown, options: { persist?: boolean } = {}): Theme {
  const theme = normalizeTheme(value);
  const resolved = resolvedTheme(theme);
  const root = document.documentElement;

  root.classList.toggle("dark", resolved === "dark");
  root.classList.toggle("light", resolved === "light");
  root.dataset.theme = theme;
  root.style.colorScheme = resolved;
  document.body.dataset.theme = theme;

  if (options.persist !== false) {
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // localStorage can be unavailable in hardened webviews; the class is
      // still applied for this session.
    }
  }

  detachSystemListener();
  if (theme === "system") {
    systemQuery = window.matchMedia("(prefers-color-scheme: dark)");
    systemListener = () => applyTheme("system", { persist: false });
    systemQuery.addEventListener("change", systemListener);
  }
  return theme;
}

// bootstrapTheme runs before React renders to avoid a flash of the wrong
// palette and to keep persistence independent of the status query.
export function bootstrapTheme(): Theme {
  const theme = readStoredTheme();
  applyTheme(theme, { persist: false });
  try {
    // Persist the resolved choice under the new key once, so later reads do
    // not depend on the legacy entry.
    window.localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // localStorage can be unavailable in hardened webviews.
  }
  return theme;
}
