import * as dotenv from 'dotenv';
dotenv.config();

import * as GlobalContext from '../globalContext';
import { Mastodon, Status } from '../api/mastodon';
import { Logger } from '../logging';
import * as fs from 'fs';
import { setTimeout } from 'timers/promises';

function normalizeStatus(status: Status): any {
    return {
        id: status.id,
        url: status.url,
        in_reply_to_id: status.in_reply_to_id,
        in_reply_to_account_id: status.in_reply_to_account_id,
        content: status.content,
        account: {
            id: status.account.id,
            acct: status.account.acct,
            display_name: status.account.display_name,
        },
        visibility: status.visibility,
    };
}

async function saveTree(mastodon: Mastodon, status: Status, seen: Set<string>) {
    if (seen.has(status.id)) {
        return;
    }
    seen.add(status.id);

    const tree = await mastodon.getReplyTree(status.id);
    for (const ancestor of tree.ancestors) {
        seen.add(ancestor.id);
    }

    const messages = [...tree.ancestors, status].map(normalizeStatus);
    fs.writeFileSync(`tmp/history/${status.id}.txt`, JSON.stringify({ messages }));
}

async function syncSeenIds(seenIds: Set<string>) {
    // Scan tmp/history and add IDs where `${id}.txt` already exists
    const files = fs.readdirSync('tmp/history');
    for (const file of files) {
        if (file.endsWith('.txt')) {
            const id = file.slice(0, -4);
            seenIds.add(id);
        }
    }
}

async function main() {
    const env = GlobalContext.env;
    const mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);
    const logger = new Logger('crawl-cli');

    const seen = new Set<string>();
    syncSeenIds(seen);

    let maxId = '1710848';
    while (true) {
        const notifications = await mastodon.getAllNotifications({ types: ['mention'], maxId });
        if (notifications.length === 0) {
            break;
        }

        for (const notification of notifications) {
            if (seen.has(notification.status!.id)) {
                logger.info(`Skipping ${notification.status!.id} (already seen)`);
                continue;
            }
            logger.info(`Crawling ${notification.status!.id}`);
            await saveTree(mastodon, notification.status!, seen);
            if (maxId === undefined || maxId > notification.id) {
                maxId = notification.id;
            }
            await setTimeout(500);
        }
    }
} 

main();
