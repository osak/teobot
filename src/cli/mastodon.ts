import * as dotenv from 'dotenv';
dotenv.config();

import { Mastodon } from '../api/mastodon';
import * as GlobalContext from '../globalContext';
import * as readline from 'readline/promises';
import { stripHtmlTags } from '../messageUtil';
import * as fs from 'fs';

class MastodonCli {
    readonly mastodon: Mastodon

    constructor(env: GlobalContext.Env) {
        this.mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);
    }

    async runCommand(commandStr: string) {
        const [command, ...rest] = commandStr.split(/\s+/);
        switch (command) {
            case 'verify':
                console.log(await this.mastodon.verifyCredentials());
                break;
            case 'status': {
                const id = rest[0];
                const status = await this.mastodon.getStatus(id);
                console.log(`${status.account.acct}: ${status.content}`);
                break;
            }
            case 'replies': {
                const mentions = await this.mastodon.getAllNotifications(['mention']);
                for (const mention of mentions) {
                    const status = mention.status!;
                    console.log(`${mention.id}: ${status.account.acct}: ${stripHtmlTags(status.content)}`);
                }
                break;
            }
            case 'reply_tree': {
                const id = rest[0];
                const context = await this.mastodon.getReplyTree(id);
                for (const status of context.ancestors) {
                    console.log(`${status.account.acct}: ${stripHtmlTags(status.content)}`);
                }
                break;
            }
            case 'post': {
                const text = rest[0];
                await this.mastodon.postStatus(text);
                break;
            }
			case 'get_image': {
				const id = rest[0];
				const res = await this.mastodon.getImage(id);
				console.log(JSON.stringify(res, undefined, 2));
				break;
			}
			case 'post_with_image': {
				const [text, path] = rest;
				const buffer = fs.readFileSync(path);
				const res = await this.mastodon.uploadImage(buffer);
				console.log(JSON.stringify(res, undefined, 2));
				await this.mastodon.postStatus(text, { mediaIds: [res.id] });
				break;
			}
        }
    }

    async runRepl() {
        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
        });

        while (true) {
            const command = await rl.question('> ');
            await this.runCommand(command);
        }
    }
}

new MastodonCli(GlobalContext.env).runRepl();
