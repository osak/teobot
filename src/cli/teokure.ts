import * as dotenv from 'dotenv';
dotenv.config();

import { Mastodon, Status } from '../api/mastodon';
import * as GlobalContext from '../globalContext';
import * as readline from 'readline/promises';
import { AssistantMessage, ChatGPT, ChatResponse, Message, UserMessage } from '../api/chatgpt';
import { Result, err, ok, withRetry } from '../util';
import { Logger } from '../logging';
import { setTimeout } from 'timers/promises';
import { readFile, writeFile } from 'fs/promises';
import { normalizeStatusContent } from '../messageUtil';
import { TeobotService } from '../service/teobotService';
import { JmaApi } from '../api/jma';
import { DallE } from '../api/dalle';
import { TextSplitService } from '../service/textSplitService';

interface State {
    lastNotificationId?: string;
}

const HISTORY_CHARS_LIMIT = 1000;

class TeokureCli {
    private readonly logger: Logger = Logger.createLogger('teokure-cli');
	private readonly teobotService: TeobotService;
    private readonly mastodon: Mastodon
	private readonly textSplitService: TextSplitService;
    private myAccountId?: string;
    private state: State;
    private dataPath: string;
    private dryRun: boolean;

    constructor(env: GlobalContext.Env) {
		const chatGpt = new ChatGPT(env.CHAT_GPT_API_KEY);
        this.teobotService = new TeobotService(
			chatGpt,
			new JmaApi(),
			new DallE(env.CHAT_GPT_API_KEY),
		);
        this.mastodon = new Mastodon(env.MASTODON_BASE_URL, env.MASTODON_CLIENT_KEY, env.MASTODON_CLIENT_SECRET, env.MASTODON_ACCESS_TOKEN);
		this.textSplitService = new TextSplitService(chatGpt);
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

        const context = this.teobotService.newChatContext();

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
            const rawReply = await withRetry({ label: 'chat' }, () => this.teobotService.chat(context, { role: 'user', content: mentionText, name: username }));
            this.logger.info(`> Response from ChatGPT: ${rawReply.message.content}`);
			const repliesRes = await this.parseReply(rawReply);
			if (repliesRes.type == 'err') {
				throw new Error(`Failed to parse reply: ${repliesRes.value}`);
			}

			let replyToId = status.id;
			let first = true;
			for (const reply of repliesRes.value) {
				const content = reply.text.replace(/@/g, '@ ');
				let replyText;
				if (content.length > 500) {
					replyText = `@${status.account.acct} 文字数上限を超えました`;
					reply.imageUrls = [];
				} else {
					replyText = `@${status.account.acct} ${content}`;
				}
				this.logger.info(`${replyText}`);

				if (!this.dryRun) {
					if (!first) {
						await setTimeout(1000);
					}
					if (reply.imageUrls.length > 0) {
						// Download images
						const medias = await Promise.all(reply.imageUrls.map(async (url) => {
							this.logger.info(`Downloading ${url}`);
							const response = await fetch(url);
							const imageBuffer = await response.arrayBuffer();
							this.logger.info("Uploading media");
							const media = await this.mastodon.uploadImage(Buffer.from(imageBuffer));
							this.logger.info(JSON.stringify(media, undefined, 2));
							return media;
						}));
						const post = await this.mastodon.postStatus(replyText, {
							replyToId,
							mediaIds: medias.map((m) => m.id),
								visibility: status.visibility,
							sensitive: true,
						});
						replyToId = post.id;
					} else {
						const post = await this.mastodon.postStatus(replyText, {
							replyToId,
							visibility: status.visibility
						});
						replyToId = post.id;
					}
				}
				first = false;
			}
        } catch (e) {
            this.logger.error(`ChatGPT returned error: ${e}`);
            if (!this.dryRun) {
                await this.mastodon.postStatus(`@${status.account.acct} エラーが発生しました`, {
					replyToId: status.id,
					visibility: status.visibility,
				});
            }
            return;
        }
    }

	private async parseReply(reply: ChatResponse): Promise<Result<{ text: string, imageUrls: string[] }[], string>> {
		const content = reply.message.content?.replaceAll(/!?\[([^\]]+)\]\([^)]+\)/g, '$1') ?? '';
		if (content.length > 500) {
			const res = await this.textSplitService.splitText(content, Math.ceil(content.length / 450));
			if (res.type === 'ok') {
				return ok(res.value.map((p, i) => {
					if (i == 0) {
						return { text: p, imageUrls: reply.imageUrls };
					} else {
						return { text: p, imageUrls: [] };
					}
				}));
			} else {
				return err('Failed to split');
			}
		}
		return ok([{
			text: content,
			imageUrls: reply.imageUrls,
		}]);
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
                const mentions = (await withRetry({ label: 'notifications' }, () => this.mastodon.getAllNotifications({ types: ['mention'], sinceId: this.state.lastNotificationId })))
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
