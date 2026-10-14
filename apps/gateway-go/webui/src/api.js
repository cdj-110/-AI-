export class ApiError extends Error {
    code;
    detail;
    targetType;
    validRange;
    constructor(message, data = {}) {
        super(message);
        this.name = 'ApiError';
        this.code = typeof data.code === 'string' ? data.code : undefined;
        this.detail = typeof data.detail === 'string' ? data.detail : undefined;
        this.targetType = typeof data.targetType === 'string' ? data.targetType : undefined;
        this.validRange = typeof data.validRange === 'string' ? data.validRange : undefined;
    }
}
export async function api(path, options = {}) {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), options.timeoutMs ?? 20000);
    try {
        const response = await fetch(path, { ...options, signal: controller.signal });
        if (response.status === 401) {
            window.location.assign('/login?next=' + encodeURIComponent(window.location.pathname + window.location.search + window.location.hash));
            throw new Error('登录已失效');
        }
        if (!response.ok) {
            const text = (await response.text()).trim();
            let data = {};
            try {
                data = JSON.parse(text);
            }
            catch { }
            const message = typeof data.message === 'string' ? data.message : text || `请求失败 (${response.status})`;
            throw new ApiError(message, { ...data, detail: typeof data.detail === 'string' ? data.detail : text });
        }
        return await response.json();
    }
    finally {
        window.clearTimeout(timer);
    }
}
export async function loadGatewayConfig() {
    const data = await api('/api/config');
    return JSON.parse(data.content);
}
