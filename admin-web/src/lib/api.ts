// Admin API client for the Madexchanger RPC-style single-endpoint API.
// All requests are POST /api/admin with a JSON body, matching Madmail's pattern.

export interface ApiConfig {
    baseUrl: string;
    token: string;
}

export interface ApiResponse<T = unknown> {
    status: number;
    resource: string;
    body: T;
    error: string | null;
}

// --- Response types ---

export interface Stats {
    total_relayed: number;
    total_rejected: number;
    total_errors: number;
    total_bytes: number;
}

export interface MessageRecord {
    id: number;
    timestamp: string;
    mail_from: string;
    mail_to: string;
    size_bytes: number;
    status: string;
    error_message: string;
    remote_addr: string;
    downstream: string;
}

export interface RelayConfig {
    relay_mode: string;
    downstream_url: string;
    forward_path: string;
    receive_path: string;
    skip_tls_verify: boolean;
    max_body_size: number;
}

export interface RewriteRule {
    id: number;
    enabled: boolean;
    field: string;
    pattern: string;
    replacement: string;
    comment: string;
}

export interface RelayFilter {
    id: number;
    enabled: boolean;
    field: string;
    pattern: string;
    comment: string;
}

// --- RPC Client ---

export async function apiCall<T = unknown>(
    config: ApiConfig,
    resource: string,
    method: string = 'GET',
    body?: unknown
): Promise<{ data?: T; error?: string; status: number }> {
    try {
        const targetUrl = config.baseUrl.replace(/\/+$/, '') + '/api/admin';
        const payload = {
            method,
            resource,
            headers: { Authorization: `Bearer ${config.token}` },
            body: body ?? {}
        };

        const res = await fetch(targetUrl, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        const text = await res.text();
        if (!text) {
            if (res.status >= 200 && res.status < 300) {
                return { data: undefined as unknown as T, status: res.status };
            }
            return { error: `Empty response (HTTP ${res.status})`, status: res.status };
        }

        let json: ApiResponse<T>;
        try {
            json = JSON.parse(text);
        } catch {
            return { error: `Invalid JSON: ${text.substring(0, 200)}`, status: res.status };
        }

        if (json.error) {
            return { error: json.error, status: json.status };
        }
        return { data: json.body, status: json.status ?? res.status };
    } catch (e) {
        return { error: String(e), status: 0 };
    }
}

// --- Convenience Wrappers ---

export const api = {
    // Stats
    stats: (c: ApiConfig) => apiCall<Stats>(c, '/admin/stats'),

    // Messages
    messages: (c: ApiConfig, limit: number = 50) =>
        apiCall<MessageRecord[]>(c, '/admin/messages', 'GET', { limit }),

    // Config
    config: (c: ApiConfig) => apiCall<RelayConfig>(c, '/admin/config'),
    setRelayMode: (c: ApiConfig, mode: string) =>
        apiCall(c, '/admin/config', 'POST', { relay_mode: mode }),

    // Rewrite rules
    rewrites: (c: ApiConfig) => apiCall<RewriteRule[]>(c, '/admin/rewrites'),
    addRewrite: (c: ApiConfig, rule: Omit<RewriteRule, 'id'>) =>
        apiCall<RewriteRule>(c, '/admin/rewrites', 'POST', rule),
    updateRewrite: (c: ApiConfig, rule: RewriteRule) =>
        apiCall<RewriteRule>(c, '/admin/rewrites', 'PUT', rule),
    deleteRewrite: (c: ApiConfig, id: number) =>
        apiCall(c, '/admin/rewrites', 'DELETE', { id }),

    // Relay filters
    filters: (c: ApiConfig) => apiCall<RelayFilter[]>(c, '/admin/filters'),
    addFilter: (c: ApiConfig, filter: Omit<RelayFilter, 'id'>) =>
        apiCall<RelayFilter>(c, '/admin/filters', 'POST', filter),
    updateFilter: (c: ApiConfig, filter: RelayFilter) =>
        apiCall<RelayFilter>(c, '/admin/filters', 'PUT', filter),
    deleteFilter: (c: ApiConfig, id: number) =>
        apiCall(c, '/admin/filters', 'DELETE', { id }),
};
