<script lang="ts">
  import { store } from "$lib/state.svelte";
  import { Shield, Trash2, Globe } from "lucide-svelte";

  // --- Proxy form ---
  let name = $state("");
  let type = $state("socks5");
  let host = $state("");
  let username = $state("");
  let password = $state("");
  let comment = $state("");

  async function addProxy() {
    if (!host.trim()) {
      store.notify("Host is required", "err");
      return;
    }
    await store.addProxy({
      name: name.trim() || host.trim(),
      type,
      host: host.trim(),
      username: username.trim(),
      password: password.trim(),
      enabled: true,
      comment: comment.trim(),
    });
    name = "";
    host = "";
    username = "";
    password = "";
    comment = "";
  }

  async function toggleProxy(proxy: (typeof store.proxies)[0]) {
    await store.updateProxy({ ...proxy, enabled: !proxy.enabled });
  }


  async function delProxy(id: number) {
    if (!confirm("Delete this proxy and all its routes?")) return;
    await store.deleteProxy(id);
  }

  // --- Proxy route form ---
  let routeDest = $state("");
  let routeProxyId = $state(0);
  let routeComment = $state("");

  async function addRoute() {
    if (!routeDest.trim()) {
      store.notify("Destination is required", "err");
      return;
    }
    if (!routeProxyId) {
      store.notify("Select a proxy", "err");
      return;
    }
    await store.addProxyRoute({
      destination: routeDest.trim(),
      proxy_id: routeProxyId,
      comment: routeComment.trim(),
    });
    routeDest = "";
    routeComment = "";
  }

  async function delRoute(id: number) {
    if (!confirm("Delete this proxy route?")) return;
    await store.deleteProxyRoute(id);
  }
</script>

<!-- Proxies Section -->
<div class="bg-surface-2 rounded-lg p-4 border border-border mb-4">
  <h3 class="text-sm font-medium mb-2 flex items-center gap-1.5">
    <Shield size={14} class="text-text-2" />
    Outbound Proxies
  </h3>
  <p class="text-xs text-text-2 mb-4">
    Configure SOCKS5 or HTTP/HTTPS CONNECT proxies for outgoing email
    forwarding. Use <strong>Proxy Routes</strong> below to map destinations to proxies.
  </p>

  <!-- Add Proxy Form -->
  <div class="flex gap-2 mb-4 flex-wrap items-center">
    <input
      bind:value={name}
      placeholder="Name (optional)"
      class="flex-[0.6] min-w-[100px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <select
      bind:value={type}
      class="px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text focus:border-accent outline-none transition min-w-[90px]"
    >
      <option value="socks5">SOCKS5</option>
      <option value="http">HTTP</option>
      <option value="https">HTTPS</option>
    </select>
    <input
      bind:value={host}
      placeholder="host:port"
      class="flex-1 min-w-[150px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <input
      bind:value={username}
      placeholder="Username"
      class="flex-[0.5] min-w-[80px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <input
      bind:value={password}
      type="password"
      placeholder="Password"
      class="flex-[0.5] min-w-[80px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <button
      onclick={addProxy}
      class="px-3 py-1.5 text-xs border border-accent/30 rounded-lg hover:bg-accent/10 text-accent transition-colors font-medium"
    >
      + Add
    </button>
  </div>

  <!-- Proxies Table -->
  <div class="overflow-x-auto">
    <table class="w-full text-xs">
      <thead>
        <tr class="text-text-2 text-left">
          <th class="pb-2 pr-3 font-medium">On</th>
          <th class="pb-2 pr-3 font-medium">Name</th>
          <th class="pb-2 pr-3 font-medium">Type</th>
          <th class="pb-2 pr-3 font-medium">Host</th>
          <th class="pb-2 pr-3 font-medium">Auth</th>
          <th class="pb-2 pr-3 font-medium">Comment</th>
          <th class="pb-2 font-medium w-8"></th>
        </tr>
      </thead>
      <tbody>
        {#if store.proxies.length === 0}
          <tr>
            <td colspan="7" class="text-center text-text-2 py-8">
              No proxies configured
            </td>
          </tr>
        {:else}
          {#each store.proxies as proxy}
            <tr class="border-t border-border/50 hover:bg-accent/[0.02]">
              <td class="py-2 pr-3">
                <button
                  onclick={() => toggleProxy(proxy)}
                  aria-label="Toggle proxy {proxy.enabled ? 'off' : 'on'}"
                  class="w-9 h-5 rounded-full relative transition-colors {proxy.enabled
                    ? 'bg-accent-dim'
                    : 'bg-surface-3 border border-border'}"
                >
                  <span
                    class="absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform {proxy.enabled
                      ? 'left-[18px]'
                      : 'left-0.5'}"
                  ></span>
                </button>
              </td>
              <td class="py-2 pr-3 font-medium">{proxy.name}</td>
              <td class="py-2 pr-3">
                <span
                  class="inline-block px-2 py-0.5 rounded text-[10px] font-medium bg-accent/15 text-accent"
                >
                  {proxy.type}
                </span>
              </td>
              <td class="py-2 pr-3 font-mono">{proxy.host}</td>
              <td class="py-2 pr-3 text-text-2">
                {proxy.username ? "✓" : "—"}
              </td>
              <td class="py-2 pr-3 text-text-2">{proxy.comment}</td>
              <td class="py-2">
                <button
                  onclick={() => delProxy(proxy.id)}
                  class="p-1 text-text-2 hover:text-danger transition-colors rounded"
                  title="Delete"
                >
                  <Trash2 size={12} />
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

<!-- Proxy Routes Section -->
<div class="bg-surface-2 rounded-lg p-4 border border-border">
  <h3 class="text-sm font-medium mb-2 flex items-center gap-1.5">
    <Globe size={14} class="text-text-2" />
    Proxy Routes
    <span class="text-[10px] text-warning font-normal"
      >(destination → proxy mapping)</span
    >
  </h3>
  <p class="text-xs text-text-2 mb-4">
    Route specific destinations through specific proxies. Use patterns with
    <code class="bg-surface-3 px-1 rounded">*</code> wildcards (e.g.,
    <code class="bg-surface-3 px-1 rounded">*.example.org</code>). Unmatched
    destinations use the default proxy or direct connection.
  </p>

  <!-- Add Route Form -->
  <div class="flex gap-2 mb-4 flex-wrap items-center">
    <input
      bind:value={routeDest}
      placeholder="Destination (e.g., 10.0.0.* or *.example.org)"
      class="flex-1 min-w-[200px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <select
      bind:value={routeProxyId}
      class="px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text focus:border-accent outline-none transition min-w-[140px]"
    >
      <option value={0}>Select proxy...</option>
      {#each store.proxies as p}
        <option value={p.id}>{p.name} ({p.type}://{p.host})</option>
      {/each}
    </select>
    <input
      bind:value={routeComment}
      placeholder="Comment"
      class="flex-[0.5] min-w-[100px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <button
      onclick={addRoute}
      class="px-3 py-1.5 text-xs border border-accent/30 rounded-lg hover:bg-accent/10 text-accent transition-colors font-medium"
    >
      + Add
    </button>
  </div>

  <!-- Routes Table -->
  <div class="overflow-x-auto">
    <table class="w-full text-xs">
      <thead>
        <tr class="text-text-2 text-left">
          <th class="pb-2 pr-3 font-medium">Destination</th>
          <th class="pb-2 pr-3 font-medium">Proxy</th>
          <th class="pb-2 pr-3 font-medium">Comment</th>
          <th class="pb-2 font-medium w-8"></th>
        </tr>
      </thead>
      <tbody>
        {#if store.proxyRoutes.length === 0}
          <tr>
            <td colspan="4" class="text-center text-text-2 py-8">
              No proxy routes configured — all traffic uses direct connection
            </td>
          </tr>
        {:else}
          {#each store.proxyRoutes as route}
            <tr class="border-t border-border/50 hover:bg-accent/[0.02]">
              <td class="py-2 pr-3 font-mono">{route.destination}</td>
              <td class="py-2 pr-3">
                <span
                  class="inline-block px-2 py-0.5 rounded text-[10px] font-medium bg-accent/15 text-accent"
                >
                  {route.proxy_name || `#${route.proxy_id}`}
                </span>
              </td>
              <td class="py-2 pr-3 text-text-2">{route.comment}</td>
              <td class="py-2">
                <button
                  onclick={() => delRoute(route.id)}
                  class="p-1 text-text-2 hover:text-danger transition-colors rounded"
                  title="Delete"
                >
                  <Trash2 size={12} />
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
