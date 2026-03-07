// Reactive state store for the Madexchanger admin dashboard.
// Follows the same pattern as Madmail's state.svelte.ts.

import { api, type ApiConfig, type Stats, type RelayConfig, type MessageRecord, type RewriteRule, type RelayFilter, type Proxy, type ProxyRoute } from './api';

function createStore() {
    // Connection
    let baseUrl = $state(localStorage.getItem('mxe_url') || '');
    let token = $state(localStorage.getItem('mxe_token') || '');
    let connected = $state(false);
    let connecting = $state(false);
    let connectError = $state('');
    let refreshing = $state(false);

    // Data
    let stats = $state<Stats | null>(null);
    let config = $state<RelayConfig | null>(null);
    let messages = $state<MessageRecord[]>([]);
    let rewrites = $state<RewriteRule[]>([]);
    let filters = $state<RelayFilter[]>([]);
    let proxies = $state<Proxy[]>([]);
    let proxyRoutes = $state<ProxyRoute[]>([]);

    // Toast
    let toast = $state('');
    let toastType = $state<'ok' | 'err'>('ok');
    let toastTimer: ReturnType<typeof setTimeout> | null = null;

    function cfg(): ApiConfig {
        return { baseUrl, token };
    }

    function notify(msg: string, type: 'ok' | 'err' = 'ok') {
        toast = msg;
        toastType = type;
        if (toastTimer) clearTimeout(toastTimer);
        toastTimer = setTimeout(() => { toast = ''; }, 3000);
    }

    async function connect() {
        connecting = true;
        connectError = '';
        try {
            const res = await api.stats(cfg());
            if (res.error) {
                connectError = res.error;
                connected = false;
            } else {
                stats = res.data ?? null;
                connected = true;
                localStorage.setItem('mxe_url', baseUrl);
                localStorage.setItem('mxe_token', token);
                await refresh();
            }
        } catch (e) {
            connectError = String(e);
            connected = false;
        } finally {
            connecting = false;
        }
    }

    function disconnect() {
        connected = false;
        stats = null;
        config = null;
        messages = [];
        rewrites = [];
        filters = [];
        proxies = [];
        proxyRoutes = [];
    }

    async function refresh() {
        refreshing = true;
        try {
            const [s, c, m, rw, fl, px, pr] = await Promise.all([
                api.stats(cfg()),
                api.config(cfg()),
                api.messages(cfg(), 50),
                api.rewrites(cfg()),
                api.filters(cfg()),
                api.proxies(cfg()),
                api.proxyRoutes(cfg()),
            ]);
            if (s.data) stats = s.data;
            if (c.data) config = c.data;
            if (m.data) messages = m.data;
            if (rw.data) rewrites = rw.data;
            if (fl.data) filters = fl.data;
            if (px.data) proxies = px.data;
            if (pr.data) proxyRoutes = pr.data;
        } catch (e) {
            notify(String(e), 'err');
        } finally {
            refreshing = false;
        }
    }

    async function setRelayMode(mode: string) {
        const res = await api.setRelayMode(cfg(), mode);
        if (res.error) {
            notify(res.error, 'err');
        } else {
            if (config) config = { ...config, relay_mode: mode };
            notify(`Relay mode set to: ${mode}`);
        }
    }

    async function addRewrite(rule: Omit<RewriteRule, 'id'>) {
        const res = await api.addRewrite(cfg(), rule);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Rewrite rule added');
        await loadRewrites();
    }

    async function updateRewrite(rule: RewriteRule) {
        const res = await api.updateRewrite(cfg(), rule);
        if (res.error) { notify(res.error, 'err'); return; }
        await loadRewrites();
    }

    async function deleteRewrite(id: number) {
        const res = await api.deleteRewrite(cfg(), id);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Rule deleted');
        await loadRewrites();
    }

    async function addFilter(filter: Omit<RelayFilter, 'id'>) {
        const res = await api.addFilter(cfg(), filter);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Relay filter added');
        await loadFilters();
    }

    async function updateFilter(filter: RelayFilter) {
        const res = await api.updateFilter(cfg(), filter);
        if (res.error) { notify(res.error, 'err'); return; }
        await loadFilters();
    }

    async function deleteFilter(id: number) {
        const res = await api.deleteFilter(cfg(), id);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Filter deleted');
        await loadFilters();
    }

    // --- Proxy CRUD ---

    async function addProxy(proxy: Omit<Proxy, 'id'>) {
        const res = await api.addProxy(cfg(), proxy);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Proxy added');
        await loadProxies();
    }

    async function updateProxy(proxy: Proxy) {
        const res = await api.updateProxy(cfg(), proxy);
        if (res.error) { notify(res.error, 'err'); return; }
        await loadProxies();
    }

    async function deleteProxy(id: number) {
        const res = await api.deleteProxy(cfg(), id);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Proxy deleted');
        await loadProxies();
        await loadProxyRoutes();
    }

    async function addProxyRoute(route: Omit<ProxyRoute, 'id' | 'proxy_name'>) {
        const res = await api.addProxyRoute(cfg(), route);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Proxy route added');
        await loadProxyRoutes();
    }

    async function deleteProxyRoute(id: number) {
        const res = await api.deleteProxyRoute(cfg(), id);
        if (res.error) { notify(res.error, 'err'); return; }
        notify('Route deleted');
        await loadProxyRoutes();
    }

    async function loadRewrites() {
        const res = await api.rewrites(cfg());
        if (res.data) rewrites = res.data;
    }

    async function loadFilters() {
        const res = await api.filters(cfg());
        if (res.data) filters = res.data;
    }

    async function loadProxies() {
        const res = await api.proxies(cfg());
        if (res.data) proxies = res.data;
    }

    async function loadProxyRoutes() {
        const res = await api.proxyRoutes(cfg());
        if (res.data) proxyRoutes = res.data;
    }

    function fmtBytes(b: number): string {
        if (!b || b === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(b) / Math.log(1024));
        return (b / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    }

    return {
        get baseUrl() { return baseUrl; },
        set baseUrl(v: string) { baseUrl = v; },
        get token() { return token; },
        set token(v: string) { token = v; },
        get connected() { return connected; },
        get connecting() { return connecting; },
        get connectError() { return connectError; },
        get refreshing() { return refreshing; },
        get stats() { return stats; },
        get config() { return config; },
        get messages() { return messages; },
        get rewrites() { return rewrites; },
        get filters() { return filters; },
        get proxies() { return proxies; },
        get proxyRoutes() { return proxyRoutes; },
        get toast() { return toast; },
        get toastType() { return toastType; },
        connect,
        disconnect,
        refresh,
        notify,
        setRelayMode,
        addRewrite,
        updateRewrite,
        deleteRewrite,
        addFilter,
        updateFilter,
        deleteFilter,
        addProxy,
        updateProxy,
        deleteProxy,
        addProxyRoute,
        deleteProxyRoute,
        fmtBytes,
    };
}

export const store = createStore();
