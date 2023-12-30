import * as dotenv from 'dotenv';
dotenv.config();

import { Mastodon } from '../mastodon';
import * as GlobalContext from '../globalContext';
import * as readline from 'readline/promises';
import { stripHtmlTags } from '../util';

class MastodonCli {
    readonly mastodon: Mastodon

    constructor(env: GlobalContext.Env) {
        this.mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);
    }

    async runCommand(commandStr: string) {
        const [command, rest] = commandStr.split(/\s+/, 2);
        switch (command) {
            case 'verify':
                console.log(await this.mastodon.verifyCredentials());
                break;
            case 'status': {
                const id = rest;
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
                const id = rest;
                const context = await this.mastodon.getReplyTree(id);
                for (const status of context.ancestors) {
                    console.log(`${status.account.acct}: ${stripHtmlTags(status.content)}`);
                }
                break;
            }
            case 'post': {
                const text = rest;
                await this.mastodon.postStatus(text);
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