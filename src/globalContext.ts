import { ChatGPT } from "./chatgpt";

export interface Env {
    CHAT_GPT_API_KEY: string;
    MASTODON_BASE_URL: string;
    MASTODON_CLIENT_KEY: string;
    MASTODON_CLIENT_SECRET: string;
    MASTODON_ACCESS_TOKEN: string;
    TEOKURE_STORAGE_PATH: string;
}

export const env: Env = {
    CHAT_GPT_API_KEY: ensureEnv('CHAT_GPT_API_KEY'),
    MASTODON_BASE_URL: ensureEnv('MASTODON_BASE_URL'),
    MASTODON_CLIENT_KEY: ensureEnv('MASTODON_CLIENT_KEY'),
    MASTODON_CLIENT_SECRET: ensureEnv('MASTODON_CLIENT_SECRET'),
    MASTODON_ACCESS_TOKEN: ensureEnv('MASTODON_ACCESS_TOKEN'),
    TEOKURE_STORAGE_PATH: ensureEnv('TEOKURE_STORAGE_PATH'),
};

export const chatGPT = new ChatGPT(env.CHAT_GPT_API_KEY);

function ensureEnv(name: string): string {
    return ensure(process.env[name], name);
}

function ensure<T>(t: T | undefined, name: string): T {
    if (typeof t === 'undefined') {
        throw new Error(`${name} must exist`);
    }
    return t;
}