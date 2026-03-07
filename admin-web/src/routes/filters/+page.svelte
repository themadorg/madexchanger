<script lang="ts">
  import { store } from "$lib/state.svelte";
  import { Filter, Trash2 } from "lucide-svelte";

  let field = $state("domain");
  let pattern = $state("");
  let comment = $state("");

  async function add() {
    if (!pattern.trim()) {
      store.notify("Pattern is required", "err");
      return;
    }
    await store.addFilter({
      enabled: true,
      field,
      pattern: pattern.trim(),
      comment: comment.trim(),
    });
    pattern = "";
    comment = "";
  }

  async function toggle(filter: (typeof store.filters)[0]) {
    await store.updateFilter({ ...filter, enabled: !filter.enabled });
  }

  async function del(id: number) {
    if (!confirm("Delete this relay filter?")) return;
    await store.deleteFilter(id);
  }
</script>

<div class="bg-surface-2 rounded-lg p-4 border border-border">
  <h3 class="text-sm font-medium mb-2 flex items-center gap-1.5">
    <Filter size={14} class="text-text-2" />
    Relay Filters
    <span class="text-[10px] text-warning font-normal"
      >(active in "Relay Selected" mode)</span
    >
  </h3>
  <p class="text-xs text-text-2 mb-4">
    Only messages matching at least one enabled filter are relayed. Filters are
    inactive in "Relay All" mode.
  </p>

  {#if store.config?.relay_mode === "all"}
    <div
      class="mb-4 px-3 py-2 bg-warning/10 border border-warning/20 rounded-lg text-warning text-xs"
    >
      Relay mode is currently <strong>Relay All</strong> — filters below are not
      active. Switch to "Relay Selected" to enable filtering.
    </div>
  {/if}

  <!-- Add Form -->
  <div class="flex gap-2 mb-4 flex-wrap items-center">
    <select
      bind:value={field}
      class="px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text focus:border-accent outline-none transition min-w-[110px]"
    >
      <option value="mail_from">Mail From</option>
      <option value="mail_to">Mail To</option>
      <option value="domain">Domain</option>
    </select>
    <input
      bind:value={pattern}
      placeholder="Pattern (e.g., example.org or *@example.org)"
      class="flex-1 min-w-[200px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <input
      bind:value={comment}
      placeholder="Comment"
      class="flex-[0.7] min-w-[100px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <button
      onclick={add}
      class="px-3 py-1.5 text-xs border border-accent/30 rounded-lg hover:bg-accent/10 text-accent transition-colors font-medium"
    >
      + Add
    </button>
  </div>

  <!-- Filters Table -->
  <div class="overflow-x-auto">
    <table class="w-full text-xs">
      <thead>
        <tr class="text-text-2 text-left">
          <th class="pb-2 pr-3 font-medium">On</th>
          <th class="pb-2 pr-3 font-medium">Field</th>
          <th class="pb-2 pr-3 font-medium">Pattern</th>
          <th class="pb-2 pr-3 font-medium">Comment</th>
          <th class="pb-2 font-medium w-8"></th>
        </tr>
      </thead>
      <tbody>
        {#if store.filters.length === 0}
          <tr>
            <td colspan="5" class="text-center text-text-2 py-8">
              No relay filters configured
            </td>
          </tr>
        {:else}
          {#each store.filters as filter}
            <tr class="border-t border-border/50 hover:bg-accent/[0.02]">
              <td class="py-2 pr-3">
                <button
                  onclick={() => toggle(filter)}
                  aria-label="Toggle filter {filter.enabled ? 'off' : 'on'}"
                  class="w-9 h-5 rounded-full relative transition-colors {filter.enabled
                    ? 'bg-accent-dim'
                    : 'bg-surface-3 border border-border'}"
                >
                  <span
                    class="absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform {filter.enabled
                      ? 'left-[18px]'
                      : 'left-0.5'}"
                  ></span>
                </button>
              </td>
              <td class="py-2 pr-3">
                <span
                  class="inline-block px-2 py-0.5 rounded text-[10px] font-medium bg-accent/15 text-accent"
                >
                  {filter.field}
                </span>
              </td>
              <td class="py-2 pr-3 font-mono">{filter.pattern}</td>
              <td class="py-2 pr-3 text-text-2">{filter.comment}</td>
              <td class="py-2">
                <button
                  onclick={() => del(filter.id)}
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
