import { ChatGPT } from "./api/chatgpt";
import { z } from 'zod';
import * as fs from 'fs';

const Env = z.object({
    CHAT_GPT_API_KEY: z.string(),
    MASTODON_BASE_URL: z.string(),
    MASTODON_CLIENT_KEY: z.string(),
    MASTODON_CLIENT_SECRET: z.string(),
    MASTODON_ACCESS_TOKEN: z.string(),
    TEOKURE_STORAGE_PATH: z.string(),
    HISTORY_STORAGE_PATH: z.string(),
    BUILD_TIMESTAMP: z.number(),
}).required();

export type Env = z.infer<typeof Env>;

export const env = loadEnv();
export const chatGPT = new ChatGPT(env.CHAT_GPT_API_KEY);

function loadEnv(): Env {
    const envJson = fs.readFileSync('env.json').toString();
    return Env.parse(JSON.parse(envJson));
}
