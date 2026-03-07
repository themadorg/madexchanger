<script lang="ts">
  import { store } from "$lib/state.svelte";
  import {
    ArrowUpRight,
    ArrowDownLeft,
    AlertCircle,
    HardDrive,
    Clock,
    Mail,
  } from "lucide-svelte";

  function fmtTime(ts: string): string {
    if (!ts) return "—";
    try {
      const d = new Date(ts);
      if (isNaN(d.getTime())) return ts;
      return d.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });
    } catch {
      return ts;
    }
  }

  function statusClass(s: string): string {
    if (s === "ok") return "bg-success/15 text-success";
    if (s === "error") return "bg-danger/15 text-danger";
    return "bg-warning/15 text-warning";
  }

  function trunc(s: string, n: number): string {
    return s && s.length > n ? s.slice(0, n) + "…" : s || "";
  }
</script>

{#snippet statCard(Icon: any, label: string, value: string, color?: string)}
  <div class="bg-surface-2 rounded-lg p-3 border border-border">
    <div class="flex items-center gap-1.5 text-text-2 text-xs mb-1">
      <Icon size={12} />
      {label}
    </div>
    <div class="text-xl font-semibold {color || ''}">{value}</div>
  </div>
{/snippet}

<!-- Relay Mode -->
<div class="bg-surface-2 rounded-lg p-4 border border-border mb-4">
  <h3 class="text-sm font-medium mb-3 flex items-center gap-1.5">
    <span
      class="w-1.5 h-1.5 rounded-full {store.config?.relay_mode === 'all'
        ? 'bg-success'
        : 'bg-warning'}"
    ></span>
    Relay Mode
  </h3>
  <div class="flex items-center gap-3 flex-wrap">
    <div class="flex border border-border rounded-lg overflow-hidden">
      <button
        onclick={() => store.setRelayMode("all")}
        class="px-5 py-2 text-xs font-medium transition-colors {store.config
          ?.relay_mode === 'all'
          ? 'bg-accent-dim text-white'
          : 'bg-surface-3 text-text-2 hover:bg-surface-2'}"
      >
        Relay All
      </button>
      <button
        onclick={() => store.setRelayMode("selected")}
        class="px-5 py-2 text-xs font-medium transition-colors {store.config
          ?.relay_mode === 'selected'
          ? 'bg-accent-dim text-white'
          : 'bg-surface-3 text-text-2 hover:bg-surface-2'}"
      >
        Relay Selected
      </button>
    </div>
    <span class="text-xs text-text-2">
      {store.config?.relay_mode === "all"
        ? "All incoming emails are forwarded to the downstream server."
        : "Only emails matching relay filter rules are forwarded."}
    </span>
  </div>
</div>

<!-- Stats Grid -->
<div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-5">
  {@render statCard(
    ArrowUpRight,
    "Relayed",
    store.stats?.total_relayed?.toLocaleString() ?? "—",
    "text-success",
  )}
  {@render statCard(
    ArrowDownLeft,
    "Rejected",
    store.stats?.total_rejected?.toLocaleString() ?? "—",
    "text-warning",
  )}
  {@render statCard(
    AlertCircle,
    "Errors",
    store.stats?.total_errors?.toLocaleString() ?? "—",
    "text-danger",
  )}
  {@render statCard(
    HardDrive,
    "Total Data",
    store.stats ? store.fmtBytes(store.stats.total_bytes) : "—",
  )}
</div>

<!-- Config Info -->
{#if store.config}
  <div class="bg-surface-2 rounded-lg p-4 border border-border mb-4">
    <h3 class="text-sm font-medium mb-3 flex items-center gap-1.5">
      <Mail size={14} class="text-text-2" />
      Relay Configuration
    </h3>
    <div class="grid grid-cols-2 sm:grid-cols-3 gap-3 text-xs">
      <div>
        <span class="text-text-2">Downstream</span>
        <div
          class="font-mono mt-0.5 truncate"
          title={store.config.downstream_url}
        >
          {store.config.downstream_url}
        </div>
      </div>
      <div>
        <span class="text-text-2">Forward Path</span>
        <div class="font-mono mt-0.5">{store.config.forward_path}</div>
      </div>
      <div>
        <span class="text-text-2">Receive Path</span>
        <div class="font-mono mt-0.5">{store.config.receive_path}</div>
      </div>
      <div>
        <span class="text-text-2">Skip TLS Verify</span>
        <div class="mt-0.5">{store.config.skip_tls_verify ? "Yes" : "No"}</div>
      </div>
      <div>
        <span class="text-text-2">Max Body Size</span>
        <div class="mt-0.5">
          {store.fmtBytes(store.config.max_body_size)}
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Recent Messages -->
<div class="bg-surface-2 rounded-lg p-4 border border-border">
  <h3 class="text-sm font-medium mb-3 flex items-center gap-1.5">
    <Clock size={14} class="text-text-2" />
    Recent Messages
  </h3>
  <div class="overflow-x-auto">
    <table class="w-full text-xs">
      <thead>
        <tr class="text-text-2 text-left">
          <th class="pb-2 pr-3 font-medium whitespace-nowrap">Time</th>
          <th class="pb-2 pr-3 font-medium whitespace-nowrap">From</th>
          <th class="pb-2 pr-3 font-medium whitespace-nowrap">To</th>
          <th class="pb-2 pr-3 font-medium whitespace-nowrap">Size</th>
          <th class="pb-2 pr-3 font-medium whitespace-nowrap">Status</th>
          <th class="pb-2 font-medium whitespace-nowrap">Downstream</th>
        </tr>
      </thead>
      <tbody>
        {#if store.messages.length === 0}
          <tr>
            <td colspan="6" class="text-center text-text-2 py-8">
              No messages yet
            </td>
          </tr>
        {:else}
          {#each store.messages as msg}
            <tr class="border-t border-border/50 hover:bg-accent/[0.02]">
              <td class="py-2 pr-3 font-mono whitespace-nowrap"
                >{fmtTime(msg.timestamp)}</td
              >
              <td
                class="py-2 pr-3 font-mono truncate max-w-[180px]"
                title={msg.mail_from}>{trunc(msg.mail_from, 28)}</td
              >
              <td
                class="py-2 pr-3 font-mono truncate max-w-[180px]"
                title={msg.mail_to}>{trunc(msg.mail_to, 28)}</td
              >
              <td class="py-2 pr-3 whitespace-nowrap"
                >{store.fmtBytes(msg.size_bytes)}</td
              >
              <td class="py-2 pr-3">
                <span
                  class="inline-block px-2 py-0.5 rounded text-[10px] font-semibold uppercase {statusClass(
                    msg.status,
                  )}"
                >
                  {msg.status}
                </span>
              </td>
              <td
                class="py-2 font-mono truncate max-w-[160px]"
                title={msg.downstream}>{trunc(msg.downstream, 24)}</td
              >
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
