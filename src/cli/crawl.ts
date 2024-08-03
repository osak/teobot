import * as dotenv from 'dotenv';
dotenv.config();

import * as GlobalContext from '../globalContext';
import { Mastodon, Status } from '../api/mastodon';
import * as fs from 'fs';
import { setTimeout } from 'timers/promises';
import { Logger } from '../logging';

const logger = new Logger('crawl-cli');

function buildSeenIds(): Set<string> {
    const seenIds = new Set<string>();
    for (const file of fs.readdirSync('history')) {
        if (!file.endsWith('.json')) {
            continue;
        }
        const data = JSON.parse(fs.readFileSync(`history/${file}`).toString());
        for (const status of data['messages']) {
            seenIds.add(status.id);
        }
    }

    logger.info(`Loaded ${seenIds.size} seen IDs`);
    return seenIds;
}

function cleanupStatus(status: Status): object {
    return {
        id: status.id,
        account: {
            id: status.account.id,
            username: status.account.username,
            acct: status.account.acct,
            display_name: status.account.display_name,
        },
        content: status.content,
        in_reply_to_id: status.in_reply_to_id,
        in_reply_to_account_id: status.in_reply_to_account_id,
        visibility: status.visibility,
        created_at: status.created_at,
    };
}

function saveStatusTree(path: string, tree: Status[]) {
    const data = {
        messages: tree.map((s) => cleanupStatus(s)),
    }
    fs.writeFileSync(path, JSON.stringify(data));
}

async function main() {
    const env = GlobalContext.env;
    const mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);

    const seenIds = buildSeenIds();
    let maxId: string | undefined = undefined;

    while (true) {
        await setTimeout(500);
        const statuses = await mastodon.getHomeTimeline({ maxId, limit: 40 });
        if (statuses.length === 0) {
            break;
        }
        for (const status of statuses) {
            if (!seenIds.has(status.id)) {
                seenIds.add(status.id);

                logger.info(`Saving status ${status.id}`);
                const ancestors = (await mastodon.getReplyTree(status.id)).ancestors;
                const tree = [...ancestors, status];
                saveStatusTree(`history/${status.id}.json`, tree);

                for (const ancestor of ancestors) {
                    seenIds.add(ancestor.id);
                }
                await setTimeout(500);
            }
            if (maxId == undefined || maxId > status.id) {
                maxId = status.id;
            }
        }
    }
}

main();
