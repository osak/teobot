export interface JsonApiCustom {
    headers?: () => Record<string, string>;
    checkStatus?: (code: number) => boolean;
    handleError?: (response: Response) => Promise<void>;
}

type HttpMethod = 'GET' | 'POST';

export class JsonApi {
    constructor(
        private readonly baseUrl: string,
        private readonly custom: JsonApiCustom = {}
    ) {}

    async get<T>(path: string): Promise<T> {
        return this.doCall(path, 'GET');
    }

    async post<T, B>(path: string, body: B): Promise<T> {
        return this.doCall(path, 'POST', body);
    }

    private async doCall<T, B>(path: string, method: HttpMethod, body?: B): Promise<T> {
        const url = `${this.baseUrl}${path}`;
        const response = await fetch(url, {
            headers: this.buildHeaders(),
            body: body && JSON.stringify(body),
            method,
        });
        if (!this.checkStatus(response.status)) {
            await this.handleError(response);
        }
        return await response.json() as T;
    }

    private buildHeaders(): HeadersInit {
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };

        if (this.custom.headers) {
            return {
                ...headers,
                ...this.custom.headers(),
            };
        } else {
            return headers;
        }
    }

    private checkStatus(code: number): boolean {
        if (this.custom.checkStatus) {
            return this.custom.checkStatus(code);
        } else {
            return code >= 200 && code < 300;
        }
    }

    private async handleError(response: Response): Promise<never> {
        if (this.custom.handleError) {
            this.custom.handleError(response);
        }
        const body = await response.text();
        throw new Error(`API returned error: ${body}`);
    }
}