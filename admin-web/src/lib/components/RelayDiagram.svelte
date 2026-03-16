<script lang="ts">
  import { onMount } from "svelte";
  import * as d3 from "d3";
  import type { RelayConfig, MessageRecord, Proxy, ProxyRoute } from "$lib/api";

  let {
    config = null, messages = [], proxies = [], proxyRoutes = [],
  }: {
    config?: RelayConfig | null;
    messages?: MessageRecord[];
    proxies?: Proxy[];
    proxyRoutes?: ProxyRoute[];
  } = $props();

  const COLORS = [
    "#818cf8", "#34d399", "#f472b6", "#fbbf24",
    "#38bdf8", "#fb923c", "#a78bfa", "#4ade80",
  ];
  const PROXY_CLR = "#f59e0b";

  interface Node { label: string; color: string; }

  // Determine if proxy is active: routes exist AND the referenced proxy is enabled
  let activeProxy = $derived.by<Proxy | null>(() => {
    if (proxyRoutes.length === 0) return null;
    for (const route of proxyRoutes) {
      const p = proxies.find(px => px.id === route.proxy_id && px.enabled);
      if (p) return p;
    }
    return null;
  });
  let hasProxy = $derived(!!activeProxy);

  let senders = $derived.by<Node[]>(() => {
    const s = new Set<string>();
    for (const m of messages)
      if (m.remote_addr) { const ip = m.remote_addr.replace(/:\d+$/, ""); if (ip) s.add(ip); }
    if (s.size === 0) return [{ label: "Server A", color: COLORS[0] }, { label: "Server B", color: COLORS[1] }];
    return [...s].slice(0, 4).map((ip, i) => ({ label: ip, color: COLORS[i % COLORS.length] }));
  });

  let dests = $derived.by<Node[]>(() => {
    if (config?.downstream_url) {
      const u = config.downstream_url.replace(/^https?:\/\//, "").replace(/\/.*$/, "");
      return [{ label: u, color: COLORS[3] }];
    }
    const s = new Set<string>();
    for (const m of messages)
      if (m.downstream) { const h = m.downstream.replace(/^https?:\/\//, "").replace(/\/.*$/, ""); if (h) s.add(h); }
    if (s.size === 0) return [{ label: "Dest A", color: COLORS[3] }, { label: "Dest B", color: COLORS[4] }];
    return [...s].slice(0, 4).map((h, i) => ({ label: h, color: COLORS[(i+3) % COLORS.length] }));
  });

  function trunc(s: string, n: number) { return s.length > n ? s.slice(0, n) + "…" : s; }

  // ── Layout constants ──
  // With proxy: LX(30) → HX(90) → PX(145) → DX(200), VW=230
  // No proxy:   LX(30) → HX(110) → DX(190), VW=220
  const LX = 30, PX = 145;
  let HX = $derived(hasProxy ? 90 : 110);
  let DX = $derived(hasProxy ? 200 : 190);
  let VW = $derived(hasProxy ? 230 : 220);
  const CY = 65;

  function nY(i: number, n: number): number {
    const span = 80, gap = span / (n + 1);
    return (CY - span / 2) + gap * (i + 1);
  }

  let svgEl: SVGSVGElement;
  let zoomBehavior: d3.ZoomBehavior<SVGSVGElement, unknown>;
  let zoomGroup: d3.Selection<SVGGElement, unknown, null, undefined>;

  function resetZoom() {
    if (!svgEl || !zoomBehavior) return;
    d3.select(svgEl).transition().duration(400)
      .call(zoomBehavior.transform, d3.zoomIdentity);
  }

  // ── D3 rendering ──
  function render() {
    if (!svgEl) return;

    const proxy = hasProxy;
    const sndrs = senders;
    const dsts = dests;

    const svg = d3.select(svgEl);
    svg.attr("viewBox", `0 0 ${VW} 140`);

    // Set up defs once (outside zoom group)
    if (!svg.select("defs.global-defs").node()) {
      const defs = svg.append("defs").attr("class", "global-defs");

      const hg = defs.append("radialGradient").attr("id", "hg");
      hg.append("stop").attr("offset", "0%").attr("stop-color", "var(--th-accent)").attr("stop-opacity", 0.15);
      hg.append("stop").attr("offset", "100%").attr("stop-color", "var(--th-accent)").attr("stop-opacity", 0);

      const pg = defs.append("radialGradient").attr("id", "pg");
      pg.append("stop").attr("offset", "0%").attr("stop-color", PROXY_CLR).attr("stop-opacity", 0.15);
      pg.append("stop").attr("offset", "100%").attr("stop-color", PROXY_CLR).attr("stop-opacity", 0);

      // Arrow markers
      const mkArr = (id: string, fill: string, op: number) => {
        const m = defs.append("marker").attr("id", id)
          .attr("viewBox", "0 0 6 6").attr("refX", 5).attr("refY", 3)
          .attr("markerWidth", 4).attr("markerHeight", 4).attr("orient", "auto");
        m.append("path").attr("d", "M0,0.5 L5,3 L0,5.5").attr("fill", fill).attr("opacity", op);
      };
      mkArr("arrW", "var(--th-border)", 0.4);
      mkArr("arrP", PROXY_CLR, 0.3);

      const mRet = defs.append("marker").attr("id", "arrRet")
        .attr("viewBox", "0 0 6 6").attr("refX", 1).attr("refY", 3)
        .attr("markerWidth", 4).attr("markerHeight", 4).attr("orient", "auto");
      mRet.append("path").attr("d", "M6,0.5 L1,3 L6,5.5").attr("fill", "var(--th-border)").attr("opacity", 0.3);
    }

    // Set up zoom group (once)
    if (!zoomGroup) {
      zoomGroup = svg.append("g").attr("class", "zoom-root");
      zoomBehavior = d3.zoom<SVGSVGElement, unknown>()
        .scaleExtent([0.5, 5])
        .wheelDelta((event) => -event.deltaY * (event.deltaMode === 1 ? 0.05 : event.deltaMode ? 1 : 0.002))
        .on("zoom", (event) => {
          zoomGroup.attr("transform", event.transform);
        });
      svg.call(zoomBehavior);
    }

    // Clear and redraw static content inside zoom group
    zoomGroup.selectAll(".static").remove();
    const g = zoomGroup.append("g").attr("class", "static");

    // Helper: chevron at midpoint of a line
    function drawChevron(parent: d3.Selection<SVGGElement, unknown, null, undefined>,
                         x1: number, y1: number, x2: number, y2: number,
                         color: string) {
      const mx = (x1 + x2) / 2, my = (y1 + y2) / 2;
      const angle = Math.atan2(y2 - y1, x2 - x1) * 180 / Math.PI;
      const sz = 3;
      parent.append("path")
        .attr("d", `M${-sz},${-sz * 0.7} L${0},${0} L${-sz},${sz * 0.7}`)
        .attr("transform", `translate(${mx},${my}) rotate(${angle})`)
        .attr("fill", "none").attr("stroke", color).attr("stroke-width", 0.6)
        .attr("opacity", 0.6).attr("stroke-linecap", "round");
    }

    // ── LEFT: Senders ↔ Hub (always bidirectional, solid) ──
    sndrs.forEach((_, i) => {
      const sy = nY(i, sndrs.length);
      g.append("line")
        .attr("x1", LX + 9).attr("y1", sy).attr("x2", HX - 12).attr("y2", CY)
        .attr("class", "wire-bidi");
    });

    // ── RIGHT: Hub → Dests (per-destination routing) ──
    // Determine which destinations are proxied
    const proxiedDests: boolean[] = dsts.map((dest) => {
      if (!proxy) return false;
      return proxyRoutes.some(r => {
        const p = proxies.find(px => px.id === r.proxy_id && px.enabled);
        if (!p) return false;
        // Simple glob match: convert route pattern to regex
        const pat = r.destination.replace(/\./g, "\\.").replace(/\*/g, ".*").replace(/\?/g, ".");
        return new RegExp(`^${pat}$`, "i").test(dest.label);
      });
    });

    const anyProxied = proxiedDests.some(Boolean);

    if (anyProxied) {
      // Draw trunk: Hub → Proxy
      g.append("line")
        .attr("x1", HX + 12).attr("y1", CY).attr("x2", PX - 9).attr("y2", CY)
        .attr("class", "wire-proxy");
      drawChevron(g, HX + 12, CY, PX - 9, CY, PROXY_CLR);
    }

    dsts.forEach((_, i) => {
      const dy = nY(i, dsts.length);
      if (proxiedDests[i]) {
        // Proxied: Proxy → Dest (outbound, amber dashed)
        g.append("line")
          .attr("x1", PX + 9).attr("y1", CY).attr("x2", DX - 9).attr("y2", dy)
          .attr("class", "wire-proxy");
        drawChevron(g, PX + 9, CY, DX - 9, dy, PROXY_CLR);

        // Return: Dest → Hub (direct, offset slightly to avoid overlap)
        const off = 2;
        g.append("line")
          .attr("x1", DX - 9).attr("y1", dy + off).attr("x2", HX + 12).attr("y2", CY + off)
          .attr("class", "wire-return");
        drawChevron(g, DX - 9, dy + off, HX + 12, CY + off, "var(--th-text-2)");
      } else {
        // Direct: Hub ↔ Dest (bidirectional, solid)
        g.append("line")
          .attr("x1", HX + 12).attr("y1", CY).attr("x2", DX - 9).attr("y2", dy)
          .attr("class", "wire-bidi");
      }
    });

    // ── Helper: draw server box ──
    function drawServer(parent: d3.Selection<SVGGElement, unknown, null, undefined>,
                        x: number, y: number, srv: Node) {
      const sg = parent.append("g");
      sg.append("rect")
        .attr("x", x - 8).attr("y", y - 6).attr("width", 16).attr("height", 12)
        .attr("rx", 2).attr("class", "box")
        .style("stroke", srv.color).style("stroke-opacity", 0.4);
      [-3, -0.5, 2].forEach(dy => {
        sg.append("line")
          .attr("x1", x - 5.5).attr("y1", y + dy)
          .attr("x2", dy === 2 ? x + 3 : x + 5.5).attr("y2", y + dy)
          .attr("class", "rack");
      });
      sg.append("circle").attr("cx", x + 4.5).attr("cy", y + 2).attr("r", 0.9)
        .attr("fill", srv.color).attr("opacity", 0.85);
      sg.append("text").attr("x", x).attr("y", y + 11)
        .attr("text-anchor", "middle").attr("class", "lbl")
        .attr("fill", srv.color).text(trunc(srv.label, 14));
    }

    // ── Sender nodes ──
    sndrs.forEach((srv, i) => {
      drawServer(g, LX, nY(i, sndrs.length), srv);
    });

    // ── Destination nodes ──
    dsts.forEach((srv, i) => {
      drawServer(g, DX, nY(i, dsts.length), srv);
    });

    // ── Hub ──
    g.append("circle").attr("cx", HX).attr("cy", CY).attr("r", 16).attr("fill", "url(#hg)");
    g.append("circle").attr("cx", HX).attr("cy", CY).attr("r", 16).attr("class", "pulse");
    g.append("circle").attr("cx", HX).attr("cy", CY).attr("r", 10).attr("class", "ring");
    g.append("circle").attr("cx", HX).attr("cy", CY).attr("r", 8.5).attr("class", "hub");
    g.append("path")
      .attr("d", `M${HX-2.5} ${CY-5} L${HX+1} ${CY-1} L${HX-1} ${CY-1} L${HX+2.5} ${CY+5} L${HX-1} ${CY+1} L${HX+1} ${CY+1} Z`)
      .attr("class", "bolt");
    g.append("text").attr("x", HX).attr("y", CY + 16)
      .attr("text-anchor", "middle").attr("class", "hub-lbl").text("Madexchanger");

    // ── Proxy ──
    if (anyProxied) {
      g.append("circle").attr("cx", PX).attr("cy", CY).attr("r", 12).attr("fill", "url(#pg)");
      g.append("circle").attr("cx", PX).attr("cy", CY).attr("r", 12).attr("class", "proxy-pulse");
      g.append("circle").attr("cx", PX).attr("cy", CY).attr("r", 7).attr("class", "proxy-ring");
      g.append("circle").attr("cx", PX).attr("cy", CY).attr("r", 5.5).attr("class", "proxy-fill");
      g.append("path")
        .attr("d", `M${PX} ${CY-3.5} L${PX+2.5} ${CY-1.5} L${PX+2.5} ${CY+0.5} Q${PX+1.5} ${CY+3} ${PX} ${CY+4} Q${PX-1.5} ${CY+3} ${PX-2.5} ${CY+0.5} L${PX-2.5} ${CY-1.5} Z`)
        .attr("class", "proxy-icon");
      g.append("text").attr("x", PX).attr("y", CY + 12)
        .attr("text-anchor", "middle").attr("class", "proxy-lbl")
        .text(activeProxy?.name ?? "Proxy");
      g.append("text").attr("x", PX).attr("y", CY + 16)
        .attr("text-anchor", "middle").attr("class", "proxy-sub")
        .text(`${activeProxy?.type}://${trunc(activeProxy?.host ?? "", 14)}`);
    }
  }

  // ── Bubble animation with D3 ──
  interface Bubble {
    id: number; srcIdx: number; dstIdx: number;
    dir: "ltr" | "rtl"; phase: number; t: number; color: string;
  }

  let bubbleData: Bubble[] = [];
  let uid = 0;
  let animTimer: ReturnType<typeof d3.timer>;
  let spawnTimer: ReturnType<typeof setInterval>;

  function ease(t: number): number {
    return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
  }
  function lrp(a: number, b: number, t: number) { return a + (b - a) * ease(t); }

  function bubblePos(b: Bubble): { x: number; y: number } {
    const proxy = hasProxy;
    const ly = nY(b.srcIdx % senders.length, senders.length);
    const ry = nY(b.dstIdx % dests.length, dests.length);

    if (b.dir === "ltr") {
      if (b.phase === 1) return { x: lrp(LX, HX, b.t), y: lrp(ly, CY, b.t) };
      if (proxy) {
        if (b.phase === 2) return { x: lrp(HX, PX, b.t), y: CY };
        return { x: lrp(PX, DX, b.t), y: lrp(CY, ry, b.t) };
      }
      return { x: lrp(HX, DX, b.t), y: lrp(CY, ry, b.t) };
    } else {
      if (b.phase === 1) return { x: lrp(DX, HX, b.t), y: lrp(ry, CY, b.t) };
      return { x: lrp(HX, LX, b.t), y: lrp(CY, ly, b.t) };
    }
  }

  function spawnBubble() {
    const sndrs = senders, dsts = dests;
    if (sndrs.length === 0 || dsts.length === 0) return;
    const dir: "ltr" | "rtl" = Math.random() < 0.5 ? "ltr" : "rtl";
    const si = Math.floor(Math.random() * sndrs.length);
    const di = Math.floor(Math.random() * dsts.length);
    const color = dir === "ltr" ? sndrs[si].color : dsts[di].color;
    bubbleData.push({ id: uid++, srcIdx: si, dstIdx: di, dir, phase: 1, t: 0, color });
  }

  function animateBubbles() {
    const dt = 0.015;
    const proxy = hasProxy;

    // Update
    const next: Bubble[] = [];
    for (const b of bubbleData) {
      b.t += dt;
      if (b.t >= 1) {
        const max = (b.dir === "ltr" && proxy) ? 3 : 2;
        if (b.phase < max) next.push({ ...b, phase: b.phase + 1, t: 0 });
      } else next.push(b);
    }
    bubbleData = next;

    // D3 data join — append bubbles inside the zoom group
    if (!zoomGroup) return;
    const bubs = zoomGroup.selectAll<SVGGElement, Bubble>(".bubble")
      .data(bubbleData, d => String(d.id));

    // Enter
    const enter = bubs.enter().append("g").attr("class", "bubble");
    enter.append("circle").attr("class", "b-glow").attr("r", 3.5);
    enter.append("circle").attr("class", "b-core").attr("r", 1.8);
    enter.append("line").attr("class", "b-line")
      .attr("stroke", "white").attr("stroke-width", 0.25).attr("opacity", 0.7);

    // Update + Enter
    const merged = enter.merge(bubs);
    merged.each(function(d) {
      const p = bubblePos(d);
      const el = d3.select(this);
      el.select(".b-glow").attr("cx", p.x).attr("cy", p.y).attr("fill", d.color).attr("opacity", 0.12);
      el.select(".b-core").attr("cx", p.x).attr("cy", p.y).attr("fill", d.color).attr("opacity", 0.85);
      el.select(".b-line").attr("x1", p.x - 0.8).attr("y1", p.y - 0.2)
        .attr("x2", p.x + 0.8).attr("y2", p.y - 0.2);
    });

    // Exit
    bubs.exit().remove();
  }

  onMount(() => {
    render();

    spawnBubble();
    setTimeout(spawnBubble, 400);
    setTimeout(spawnBubble, 800);
    spawnTimer = setInterval(spawnBubble, 1200);

    animTimer = d3.timer(() => animateBubbles());

    return () => {
      animTimer.stop();
      clearInterval(spawnTimer);
    };
  });

  // Re-render static parts when data changes
  $effect(() => {
    // Touch reactive deps
    void senders; void dests; void hasProxy; void activeProxy; void config; void proxyRoutes;
    render();
  });
</script>

<div class="rounded-t-lg bg-surface-2 border border-border border-b-0 p-4">
  <div class="flex items-center justify-between mb-2">
    <h3 class="text-sm font-medium text-text-2">Message Relay Flow</h3>
    <div class="flex items-center gap-2 text-[10px] text-text-2">
      <span class="flex items-center gap-1">
        <span class="inline-block w-1.5 h-1.5 rounded-full bg-accent opacity-60"></span>
        {config?.incoming_mode === "all" ? "Relay All" : config?.incoming_mode === "accept" ? "Accept (blocklist)" : "Reject (allowlist)"}
      </span>
      {#if hasProxy}
        <span class="flex items-center gap-1">
          <span class="inline-block w-1.5 h-1.5 rounded-full" style="background:{PROXY_CLR};opacity:0.6"></span>
          proxy routes
        </span>
      {/if}
    </div>
  </div>

  <div class="relative">
    <svg bind:this={svgEl} class="w-full" style="max-height: 340px; cursor: grab;"></svg>
    <button onclick={resetZoom}
      class="absolute bottom-1 right-1 text-[9px] px-1.5 py-0.5 rounded
             bg-surface-3 border border-border text-text-2
             hover:text-text hover:border-accent transition-colors"
      title="Reset zoom">
      ⟲ Reset
    </button>
  </div>
</div>

<!-- Description -->
<div class="rounded-b-lg bg-surface-3 border border-border border-t-0 px-4 py-3 mb-4">
  <div class="text-xs text-text-2 leading-relaxed space-y-1">
    {#if config?.incoming_mode !== "all"}
      <p><strong class="text-text">{config?.incoming_mode === "accept" ? "Accept mode" : "Reject mode"}</strong> — Rules on the Incoming tab control which messages are {config?.incoming_mode === "accept" ? "blocked" : "allowed"}.</p>
    {:else}
      <p><strong class="text-text">Relay All</strong> — Every incoming email is accepted and forwarded.</p>
    {/if}
    {#if hasProxy}
      <p><span class="font-mono opacity-60">↔</span> Senders <strong class="text-text">⟷</strong> Exchanger — bidirectional, direct.</p>
      <p><span class="font-mono opacity-60">→</span> Routed destinations go through <strong style="color:{PROXY_CLR}">{activeProxy?.name}</strong> (<span class="font-mono">{activeProxy?.type}://{activeProxy?.host}</span>).</p>
      <p><span class="font-mono opacity-60">↔</span> Unmatched destinations connect directly.</p>
    {:else}
      <p><span class="font-mono opacity-60">↔</span> Senders <strong class="text-text">⟷</strong> Exchanger <strong class="text-text">⟷</strong> Destinations — all direct.</p>
    {/if}
    {#if config?.downstream_url}
      <p>All traffic routes to <strong class="text-text">fixed downstream</strong> (<span class="font-mono text-accent">{config.downstream_url}</span>).</p>
    {:else}
      <p>Routing is <strong class="text-text">dynamic</strong> — delivers via HTTPS to the recipient's domain.</p>
    {/if}
  </div>
</div>

<style>
  :global(.wire) { fill: none; stroke: var(--th-border); stroke-width: 0.3; stroke-dasharray: 1.5 1; opacity: 0.4; }
  :global(.wire-bidi) { fill: none; stroke: var(--th-border); stroke-width: 0.35; opacity: 0.45; }
  :global(.wire-proxy) { fill: none; stroke: #f59e0b; stroke-width: 0.3; stroke-dasharray: 1.5 1; opacity: 0.25; }
  :global(.wire-return) { fill: none; stroke: var(--th-border); stroke-width: 0.25; stroke-dasharray: 1 1.5; opacity: 0.3; }
  :global(.box) { fill: var(--th-surface-3); stroke: var(--th-border); stroke-width: 0.5; }
  :global(.rack) { stroke: var(--th-text-2); stroke-width: 0.25; opacity: 0.35; }
  :global(.lbl) { font-size: 3.2px; font-family: "Inter", system-ui, sans-serif; font-weight: 500; }
  :global(.pulse) { fill: none; stroke: var(--th-accent); stroke-width: 0.4; opacity: 0.06; animation: breathe 3s ease-in-out infinite; }
  :global(.ring) { fill: none; stroke: var(--th-accent); stroke-width: 0.5; opacity: 0.5; }
  :global(.hub) { fill: var(--th-surface-3); }
  :global(.bolt) { fill: var(--th-accent); opacity: 0.85; }
  :global(.hub-lbl) { fill: var(--th-accent); font-size: 3.8px; font-family: "Inter", system-ui, sans-serif; font-weight: 600; }
  :global(.proxy-pulse) { fill: none; stroke: #f59e0b; stroke-width: 0.35; opacity: 0.06; animation: breathe 3s ease-in-out infinite 1.5s; }
  :global(.proxy-ring) { fill: none; stroke: #f59e0b; stroke-width: 0.5; opacity: 0.45; }
  :global(.proxy-fill) { fill: var(--th-surface-3); }
  :global(.proxy-icon) { fill: #f59e0b; opacity: 0.8; }
  :global(.proxy-lbl) { fill: #f59e0b; font-size: 3.4px; font-family: "Inter", system-ui, sans-serif; font-weight: 600; }
  :global(.proxy-sub) { fill: var(--th-text-2); font-size: 2.5px; font-family: "Inter", system-ui, sans-serif; opacity: 0.5; }
  @keyframes breathe { 0%, 100% { opacity: 0.05; } 50% { opacity: 0.12; } }
</style>
