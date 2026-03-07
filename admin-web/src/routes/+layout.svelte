<script lang="ts">
  import "./layout.css";
  import { page } from "$app/stores";
  import { base } from "$app/paths";
  import { store } from "$lib/state.svelte";
  import {
    Zap,
    Plug,
    RefreshCw,
    LogOut,
    AlertTriangle,
    LayoutDashboard,
    ArrowLeftRight,
    Filter,
    Shield,
    Sun,
    Moon,
    Github,
  } from "lucide-svelte";

  let { children } = $props();

  const appVersion =
    typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "dev";

  let canConnect = $derived(
    store.baseUrl.length > 0 && store.token.length > 0 && !store.connecting
  );

  // Theme toggle
  let isDark = $state(true);
  if (typeof localStorage !== "undefined") {
    const saved = localStorage.getItem("mxe_theme");
    isDark = saved !== "light";
  }
  $effect(() => {
    document.documentElement.setAttribute(
      "data-theme",
      isDark ? "dark" : "light"
    );
    localStorage.setItem("mxe_theme", isDark ? "dark" : "light");
  });
  function toggleTheme() {
    isDark = !isDark;
  }

  const NAV_ITEMS = [
    { href: "/", key: "Overview", icon: LayoutDashboard },
    { href: "/rewrites", key: "Rewrites", icon: ArrowLeftRight },
    { href: "/filters", key: "Filters", icon: Filter },
    { href: "/proxies", key: "Proxies", icon: Shield },
  ];

  // Auto-connect on mount
  let autoConnectDone = false;
  $effect(() => {
    if (!autoConnectDone && store.baseUrl && store.token) {
      autoConnectDone = true;
      store.connect();
    }
  });

  // Auto-refresh stats every 5s when connected
  let refreshInterval: ReturnType<typeof setInterval> | null = null;
  $effect(() => {
    if (store.connected && !refreshInterval) {
      refreshInterval = setInterval(() => store.refresh(), 10000);
    }
    if (!store.connected && refreshInterval) {
      clearInterval(refreshInterval);
      refreshInterval = null;
    }
  });

  function isActive(href: string, path: string): boolean {
    const p = base ? path.replace(base, "") || "/" : path;
    if (href === "/") return p === "/";
    return p.startsWith(href);
  }
</script>

<svelte:head>
  <title>Madexchanger Admin</title>
  <meta
    name="description"
    content="Madexchanger relay proxy administration dashboard"
  />
</svelte:head>

<!-- Toast -->
{#if store.toast}
  <div
    class="fixed top-4 end-4 z-50 px-4 py-2 rounded-lg text-sm font-medium shadow-lg transition-all
    {store.toastType === 'ok'
      ? 'bg-success/20 text-success border border-success/30'
      : 'bg-danger/20 text-danger border border-danger/30'}"
  >
    {store.toast}
  </div>
{/if}

<!-- Login Gate -->
{#if !store.connected}
  <div
    class="min-h-screen bg-surface text-text flex flex-col items-center justify-center p-4"
    style="font-family: 'Inter', system-ui, sans-serif;"
  >
    <div
      class="w-full max-w-sm p-6 bg-surface-2 rounded-xl border border-border"
    >
      <div class="flex items-center gap-2 mb-4">
        <div class="p-2 bg-accent/15 rounded-lg">
          <Zap size={18} class="text-accent" />
        </div>
        <div>
          <h1 class="text-lg font-semibold">Madexchanger</h1>
          <p class="text-text-2 text-xs">Relay proxy administration</p>
        </div>
      </div>

      <label for="url" class="block text-xs text-text-2 mb-1">Server URL</label
      >
      <input
        id="url"
        type="url"
        bind:value={store.baseUrl}
        placeholder="https://your-relay:8443"
        class="w-full mb-3 px-3 py-2 bg-surface border border-border rounded-lg text-sm text-text placeholder-text-2/40 focus:border-accent focus:ring-1 focus:ring-accent/30 outline-none transition"
      />

      <label for="tok" class="block text-xs text-text-2 mb-1">Admin Token</label
      >
      <input
        id="tok"
        type="password"
        bind:value={store.token}
        placeholder="Your admin token"
        onkeydown={(e: KeyboardEvent) => {
          if (e.key === "Enter") store.connect();
        }}
        class="w-full mb-4 px-3 py-2 bg-surface border border-border rounded-lg text-sm text-text placeholder-text-2/40 focus:border-accent focus:ring-1 focus:ring-accent/30 outline-none transition"
      />

      {#if store.connectError}
        <div class="text-danger text-xs mb-3">
          <p class="flex items-center gap-1">
            <AlertTriangle size={12} />
            {store.connectError}
          </p>
        </div>
      {/if}

      <button
        onclick={() => store.connect()}
        disabled={!canConnect}
        class="w-full py-2.5 bg-accent hover:bg-accent-dim text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2"
      >
        {#if store.connecting}
          <RefreshCw size={14} class="animate-spin" /> Connecting...
        {:else}
          <Plug size={14} /> Connect
        {/if}
      </button>
    </div>
    <a
      href="https://github.com/themadorg/madexchanger"
      target="_blank"
      rel="noopener"
      class="text-text-2/40 hover:text-text-2/70 text-[10px] mt-4 text-center flex items-center justify-center gap-1 transition-colors"
    >
      <Github size={10} />
      v{appVersion}
    </a>
  </div>
{:else}
  <!-- Authenticated Shell -->
  <div
    class="min-h-screen bg-surface text-text"
    style="font-family: 'Inter', system-ui, sans-serif;"
  >
    <div class="max-w-4xl mx-auto px-3 sm:px-4 py-4 sm:py-6">
      <!-- Header -->
      <header
        class="flex flex-wrap items-center justify-between gap-2 mb-4 sm:mb-6"
      >
        <div class="flex items-center gap-2">
          <a href="{base}/" class="p-1.5 bg-accent/15 rounded-lg">
            <Zap size={16} class="text-accent" />
          </a>
          <div>
            <div class="flex items-center gap-1.5">
              <h1 class="text-base font-semibold">Madexchanger</h1>
              <span class="text-[10px] text-text-2/50 font-mono"
                >v{appVersion}</span
              >
            </div>
            <p class="text-text-2 text-[11px] truncate max-w-[200px]">
              {store.baseUrl}
            </p>
          </div>
        </div>
        <div class="flex items-center gap-1.5">
          <button
            onclick={toggleTheme}
            class="p-1.5 text-text-2 border border-border rounded-lg hover:bg-surface-3 transition-colors"
            title={isDark ? "Light mode" : "Dark mode"}
          >
            {#if isDark}
              <Sun size={14} />
            {:else}
              <Moon size={14} />
            {/if}
          </button>
          <button
            onclick={() => store.refresh()}
            disabled={store.refreshing}
            class="p-1.5 text-text-2 border border-border rounded-lg hover:bg-surface-3 transition-colors disabled:opacity-50"
            title="Refresh"
          >
            <RefreshCw
              size={14}
              class={store.refreshing ? "animate-spin" : ""}
            />
          </button>
          <button
            onclick={() => store.disconnect()}
            class="p-1.5 text-text-2 border border-border rounded-lg hover:bg-surface-3 transition-colors"
            title="Disconnect"
          >
            <LogOut size={14} />
          </button>
        </div>
      </header>

      <!-- Navigation -->
      <nav
        class="flex gap-0.5 mb-4 sm:mb-5 border-b border-border overflow-x-auto scrollbar-hide -mx-3 px-3 sm:mx-0 sm:px-0"
      >
        {#each NAV_ITEMS as item}
          <a
            href="{base}{item.href}"
            class="px-3 py-2 text-sm transition-colors -mb-px flex items-center gap-1.5
              {isActive(item.href, $page.url.pathname)
              ? 'text-accent border-b-2 border-accent font-medium'
              : 'text-text-2 hover:text-text'}"
          >
            <item.icon size={13} />
            {item.key}
          </a>
        {/each}
      </nav>

      <!-- Page Content -->
      {@render children()}
    </div>
  </div>
{/if}
