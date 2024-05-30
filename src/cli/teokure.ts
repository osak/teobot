import * as dotenv from 'dotenv';
dotenv.config();

import { Mastodon, Status } from '../api/mastodon';
import * as GlobalContext from '../globalContext';
import * as readline from 'readline/promises';
import { AssistantMessage, ChatGPT, ChatResponse, Message, UserMessage } from '../api/chatgpt';
import { withRetry } from '../util';
import { Logger } from '../logging';
import { setTimeout } from 'timers/promises';
import { readFile, writeFile } from 'fs/promises';
import { normalizeStatusContent } from '../messageUtil';
import * as fs from 'fs';

interface State {
    lastNotificationId?: string;
}

const HISTORY_CHARS_LIMIT = 1000;

class TeokureCli {
    private readonly logger: Logger = Logger.createLogger('teokure-cli');
    private readonly chatGPT: ChatGPT
    private readonly mastodon: Mastodon
    private myAccountId?: string;
    private state: State;
    private dataPath: string;
    private dryRun: boolean;

    constructor(env: GlobalContext.Env) {
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

        const replyTree = await withRetry({ label: 'reply-tree' }, () => this.mastodon.getReplyTree(status.id));
		const history: Message[] = [];
		let chars = 0;
		for (const s of replyTree.ancestors.reverse()) {
			const normalizedContent = normalizeStatusContent(s);
			chars += normalizedContent.length;
			if (chars > HISTORY_CHARS_LIMIT) {
				break;
			}

			if (s.account.id === this.myAccountId) {
                history.unshift({ role: 'assistant', content: normalizedContent } satisfies AssistantMessage);
            } else {
                history.unshift({ role: 'user', content: normalizedContent, name: s.account.username } satisfies UserMessage);
            }

		}
        context.history = [...context.history, ...history];

        const mentionText = normalizeStatusContent(status);
        this.logger.info(`${mentionText}`);

        try {
            const username = status.account.username;
            const rawReply = await withRetry({ label: 'chat' }, () => this.chatGPT.chat(context, { role: 'user', content: mentionText, name: username }));
            this.logger.info(`> Response from ChatGPT: ${rawReply.message.content}`);
			let reply = this.parseReply(rawReply);

			if (reply.text.length > 450) {
				this.logger.info(`Reply is too long. Try to get it summarized`);
				const newReply = await withRetry({ label: 'chat' }, () => this.chatGPT.chat(rawReply.newContext, { role: 'system', content: '長すぎるので、400字以内で要約してください' }));
				this.logger.info(`> Response from ChatGPT: ${newReply.message.content}`);
				reply = this.parseReply(newReply);
			}

            const content = reply.text.replace(/@/g, '@ ');
            let replyText;
            if (content.length > 450) {
                replyText = `@${status.account.acct} 文字数上限を超えました`;
				reply.imageUrl = undefined;
            } else {
                replyText = `@${status.account.acct} ${content}`;
            }
            this.logger.info(`${replyText}`);

            if (!this.dryRun) {
				if (reply.imageUrl != undefined) {
					// Download the image
					this.logger.info(`Downloading ${reply.imageUrl}`);
					const response = await fetch(reply.imageUrl);
					const imageBuffer = await response.arrayBuffer();
					this.logger.info("Uploading media");
					const media = await this.mastodon.uploadImage(Buffer.from(imageBuffer));
					this.logger.info(JSON.stringify(media, undefined, 2));
					await this.mastodon.postStatus(replyText, status.id, [media.id], status.visibility);
				} else {
					await this.mastodon.postStatus(replyText, status.id, undefined, status.visibility);
				}
            }
        } catch (e) {
            this.logger.error(`ChatGPT returned error: ${e}`);
            if (!this.dryRun) {
                await this.mastodon.postStatus(`@${status.account.acct} エラーが発生しました`, status.id, undefined, status.visibility);
            }
            return;
        }
    }

	private parseReply(reply: ChatResponse): { imageUrl?: string, text: string } {
		const foundImage = reply.message.content!.match(/!\[.*\]\(([^\)]+)\)/d);
		if (foundImage) {
			const imageUrl = foundImage[1];
			const text = reply.message.content!.substring(0, foundImage.index) + reply.message.content!.substring(foundImage.indices![0][1]);
			return { imageUrl, text };
		} else {
			return { text: reply.message.content! };
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
                const mentions = (await withRetry({ label: 'notifications' }, () => this.mastodon.getAllNotifications(['mention'], this.state.lastNotificationId)))
                    .filter((m) => m.account.id !== this.myAccountId);
                for (const mention of mentions) {
                    try {
                        console.log(`${mention.id}: ${mention.status!.content}`);
                        await this.replyToStatus(mention.status!);
                    } catch (e) {
                        this.logger.error(`Failed to process message (id=${mention.id}): ${e}`);
                    }
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
            try {
                await this.runCommand('process_new_replies');
            } catch (e) {
                this.logger.error(`Failed to process new replies: ${e}`);
            }
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
