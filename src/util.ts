import { Logger } from './logging';
import { setTimeout } from 'timers/promises';

export type ValueOf<T> = T[keyof T];

export function padRight(s: string, minWidth: number): string {
    const diff = minWidth - s.length;
    if (diff <= 0) {
        return s;
    }

    const pad = ' '.repeat(diff);
    return `${s}${pad}`;
}

export function queryString(params: { [key: string]: string | string[] | undefined }): string {
    const fragments = Object.entries(params).map((entry) => {
        const [k, v] = entry;
        if (v === undefined) {
            return null;
        }
        if (typeof v === 'object') {
            const arr = v as string[];
            if (arr.length > 0) {
                return arr.map((val) => `${k}[]=${val}`);
            } else {
                return null;
            }
        } else {
            return `${k}=${v}`;
        }
    }).flat().filter((f) => f !== null);

    const paramsString = fragments.join('&');
    if (paramsString !== '') {
        return `?${paramsString}`;
    } else {
        return '';
    }
}

export interface RetryConfig {
    maxAttempts: number;
    label?: string;
}

export async function withRetry<T>(config: Partial<RetryConfig>, body: () => Promise<T>): Promise<T> {
    const fullConfig: RetryConfig = {
        maxAttempts: 3,
        label: '__unnamed__',
        ...config,
    };
    const logger = Logger.createLogger(`retry-${config.label}`);

    for (let i = 1; i <= fullConfig.maxAttempts; ++i) {
        try {
            return await body();
        } catch (e) {
            if (i === fullConfig.maxAttempts) {
                throw new Error(`withRetry(label=${fullConfig.label}): Retry exhausted`, { cause: e });
            } else {
                const backoff = 10;
                logger.info(`Attempt ${i} failed. Retry in ${backoff} seconds: ${e}`);
                await setTimeout(backoff * 1000);
            }
        }
    }

    throw new Error('Bug: unreachable code');
}

export interface Ok<T> {
	type: 'ok';
	value: T;
}

export interface Err<T> {
	type: 'err';
	value: T;
}

export type Result<T, E> = Ok<T> | Err<E>;

export function ok<T>(value: T): Ok<T> { return { type: 'ok', value }; }
export function err<T>(value: T): Err<T> { return { type: 'err', value }; }
