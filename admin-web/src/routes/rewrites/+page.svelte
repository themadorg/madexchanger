<script lang="ts">
  import { store } from "$lib/state.svelte";
  import { ArrowLeftRight, Trash2 } from "lucide-svelte";

  let field = $state("mail_from");
  let pattern = $state("");
  let replacement = $state("");
  let comment = $state("");

  async function add() {
    if (!pattern.trim() || !replacement.trim()) {
      store.notify("Pattern and replacement are required", "err");
      return;
    }
    await store.addRewrite({
      enabled: true,
      field,
      pattern: pattern.trim(),
      replacement: replacement.trim(),
      comment: comment.trim(),
    });
    pattern = "";
    replacement = "";
    comment = "";
  }

  async function toggle(rule: (typeof store.rewrites)[0]) {
    await store.updateRewrite({ ...rule, enabled: !rule.enabled });
  }

  async function del(id: number) {
    if (!confirm("Delete this rewrite rule?")) return;
    await store.deleteRewrite(id);
  }
</script>

<div class="bg-surface-2 rounded-lg p-4 border border-border">
  <h3 class="text-sm font-medium mb-2 flex items-center gap-1.5">
    <ArrowLeftRight size={14} class="text-text-2" />
    Endpoint Rewrite Rules
  </h3>
  <p class="text-xs text-text-2 mb-4">
    Rewrite sender, recipient, or downstream URL before forwarding. Rules are
    applied in order.
  </p>

  <!-- Add Form -->
  <div class="flex gap-2 mb-4 flex-wrap items-center">
    <select
      bind:value={field}
      class="px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text focus:border-accent outline-none transition min-w-[110px]"
    >
      <option value="mail_from">Mail From</option>
      <option value="mail_to">Mail To</option>
      <option value="downstream">Downstream</option>
    </select>
    <input
      bind:value={pattern}
      placeholder="Pattern (exact match or *wildcard)"
      class="flex-1 min-w-[140px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
    />
    <input
      bind:value={replacement}
      placeholder="Replacement"
      class="flex-1 min-w-[140px] px-2 py-1.5 text-xs bg-surface border border-border rounded-lg text-text placeholder-text-2/40 focus:border-accent outline-none transition"
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

  <!-- Rules Table -->
  <div class="overflow-x-auto">
    <table class="w-full text-xs">
      <thead>
        <tr class="text-text-2 text-left">
          <th class="pb-2 pr-3 font-medium">On</th>
          <th class="pb-2 pr-3 font-medium">Field</th>
          <th class="pb-2 pr-3 font-medium">Pattern</th>
          <th class="pb-2 pr-3 font-medium">Replacement</th>
          <th class="pb-2 pr-3 font-medium">Comment</th>
          <th class="pb-2 font-medium w-8"></th>
        </tr>
      </thead>
      <tbody>
        {#if store.rewrites.length === 0}
          <tr>
            <td colspan="6" class="text-center text-text-2 py-8">
              No rewrite rules configured
            </td>
          </tr>
        {:else}
          {#each store.rewrites as rule}
            <tr class="border-t border-border/50 hover:bg-accent/[0.02]">
              <td class="py-2 pr-3">
                <button
                  onclick={() => toggle(rule)}
                  aria-label="Toggle rule {rule.enabled ? 'off' : 'on'}"
                  class="w-9 h-5 rounded-full relative transition-colors {rule.enabled
                    ? 'bg-accent-dim'
                    : 'bg-surface-3 border border-border'}"
                >
                  <span
                    class="absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform {rule.enabled
                      ? 'left-[18px]'
                      : 'left-0.5'}"
                  ></span>
                </button>
              </td>
              <td class="py-2 pr-3">
                <span
                  class="inline-block px-2 py-0.5 rounded text-[10px] font-medium bg-accent/15 text-accent"
                >
                  {rule.field}
                </span>
              </td>
              <td class="py-2 pr-3 font-mono">{rule.pattern}</td>
              <td class="py-2 pr-3 font-mono">{rule.replacement}</td>
              <td class="py-2 pr-3 text-text-2">{rule.comment}</td>
              <td class="py-2">
                <button
                  onclick={() => del(rule.id)}
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
