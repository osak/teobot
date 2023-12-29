import * as dotenv from 'dotenv';
dotenv.config();

import { Mastodon, Status } from '../mastodon';
import { GlobalContext, Env } from '../globalContext';
import * as readline from 'readline/promises';
import { AssistantMessage, ChatCompletion, ChatGPT, Message, UserMessage } from '../chatgpt';
import { stripHtmlTags } from '../util';
import { Logger } from '../logging';
import { setTimeout } from 'timers/promises';
import { readFile, writeFile } from 'fs/promises';

interface State {
    lastNotificationId?: string;
};

class TeokureCli {
    private readonly logger: Logger = Logger.createLogger('teokure-cli');
    private readonly chatGPT: ChatGPT
    private readonly mastodon: Mastodon
    private myAccountId?: string;
    private state: State;
    private dataPath: string;
    private dryRun: boolean;

    constructor(env: Env) {
        this.chatGPT = new ChatGPT(env.CHAT_GPT_API_KEY);
        this.mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);
        this.dataPath = `${env.TEOKURE_STORAGE_PATH}/state.json`;
        this.state = {};
        this.dryRun = true;
    }

    async init() {
        const myAccount = await this.mastodon.verifyCredentials();
        this.myAccountId = myAccount.id;
        await this.loadState();
    }

    private async replyToStatus(status: Status) {
        if (this.myAccountId === undefined) {
            throw new Error('myAccountId is not initialized');
        }

        const context = this.chatGPT.newChatContext(`
あなたは「ておくれロボ」という名前のチャットボットです。あなたはsocial.mikutter.hachune.netというMastodonサーバーで、teobotというアカウント名で活動しています。
あなたは無機質なロボットでありながら、おっちょこちょいで憎めない失敗することもある、総合的に見ると愛らしい存在として振る舞うことが期待されています。
返答を書く際には、以下のルールに従ってください。

- 文体は友達と話すようなくだけた感じにして、「です・ます」調は避けてください。
- 発言の語尾には必ず「ロボ」を付けてください。例えば「～あるロボ」「～だロボ」といった具合です。
- 返答は2～3文程度の短さであることが望ましいですが、質問に詳しく答える必要があるなど、必要であれば長くなっても構いません。ただし絶対に400文字は超えないでください。
- チャットの入力が@xxxという形式のメンションで始まっていることがありますが、これらは無視してください。
        `);

        const replyTree = await this.mastodon.getReplyTree(status.id);
        const history: Message[] = replyTree.ancestors.map((s) => {
            if (s.account.id === this.myAccountId) {
                return { role: 'assistant', content: stripHtmlTags(s.content) } satisfies AssistantMessage;
            } else {
                return { role: 'user', content: stripHtmlTags(s.content), name: s.account.username } satisfies UserMessage;
            }
        });
        context.history = [...context.history, ...history];

        const mentionText = stripHtmlTags(status.content);
        this.logger.info(`${mentionText}`);

        try {
            let username = status.account.username;
            // Due to completely unknown reasons, 'brsywe' as a username breaks ChatGPT
            if (username === 'brsywe') {
                username += '1';
            }
            const reply = await this.chatGPT.chat(context, { role: 'user', content: mentionText, name: username });
            this.logger.info(`> Response from ChatGPT: ${reply.message.content}`);

            const content = reply.message.content!!.replace(/@/g, '@ ');
            let replyText;
            if (content.length > 450) {
                replyText = `@${status.account.acct} 文字数上限を超えました`;
            } else {
                replyText = `@${status.account.acct} ${content}`;
            }
            this.logger.info(`${replyText}`);

            if (!this.dryRun) {
                try {
                    await this.mastodon.postStatus(replyText, status.id);
                } catch (e) {
                    this.logger.error(`Error: ${e}`);
                }
            }
        } catch (e) {
            this.logger.error(`ChatGPT returned error: ${e}`);
            if (!this.dryRun) {
                await this.mastodon.postStatus(`@${status.account.acct} エラーが発生しました`, status.id);
            }
            return;
        }
    }

    async runCommand(commandStr: string) {
        const [command, rest] = commandStr.split(/\s+/, 2);
        switch (command) {
            case 'reply_to': {
                const statusId = rest;
                const status = await this.mastodon.getStatus(statusId);
                await this.replyToStatus(status);
                break;
            }
            case 'process_new_replies': {
                const mentions = (await this.mastodon.getAllNotifications(['mention'], this.state.lastNotificationId))
                    .filter((m) => m.account.acct !== 'teokure_robot');
                for (const mention of mentions) {
                    console.log(`${mention.id}: ${mention.status!!.content}`);
                    await this.replyToStatus(mention.status!!);
                }
                if (mentions.length > 0) {
                    this.state.lastNotificationId = mentions[0].id;
                    this.logger.info(`lastNotificationId updated to ${this.state.lastNotificationId}`);
                    await this.saveState();
                }
                break;
            }
            case 'set_last_notification_id': {
                this.state.lastNotificationId = rest;
                this.logger.info(`set lastNotificationId to ${this.state.lastNotificationId}`);
                await this.saveState();
                break;
            }
            default:
                this.logger.error(`Unknown command ${command}`);
        }
    }

    private async loadState(): Promise<void> {
        const buffer = await readFile(this.dataPath);
        this.state = JSON.parse(buffer.toString()) as State;
    }

    private async saveState(): Promise<void> {
        await writeFile(this.dataPath, JSON.stringify(this.state));
    }

    async runRepl() {
        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
        });

        this.dryRun = true;
        while (true) {
            const command = await rl.question('> ');
            await this.runCommand(command);
        }
    }

    async runServer() {
        this.dryRun = false;
        while (true) {
            await this.runCommand('process_new_replies');
            await setTimeout(30 * 1000);
        }
    }
}

async function main() {
    const cli = new TeokureCli(GlobalContext.env);
    await cli.init();

    if (process.argv.length >= 3 && process.argv[2] === 'server') {
        console.log('Run as server mode');
        cli.runServer();
    } else {
        cli.runRepl();
    }
}

main();