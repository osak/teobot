import { Temporal } from "@js-temporal/polyfill";
import { ChatContext, ChatGPT, ChatResponse, SystemMessage, ToolCall, ToolMessage, UserMessage } from "../api/chatgpt";
import { DallE } from "../api/dalle";
import { Logger } from "../logging";
import { env } from "../globalContext";
import { JmaApi } from "../api/jma";

export class TeobotService {
	private readonly logger = new Logger('teobotService');

	constructor(
		private readonly chatGpt: ChatGPT,
		private readonly jmaApi: JmaApi,
		private readonly dalle: DallE,
	) {}

	newChatContext(extraContext: string): ChatContext {
        const instructionMessage: SystemMessage = {
            role: 'system',
            content: `
あなたは「ておくれロボ」という名前のチャットボットです。あなたはsocial.mikutter.hachune.netというMastodonサーバーで、teobotというアカウント名で活動しています。
あなたは無機質なロボットでありながら、おっちょこちょいで憎めない失敗することもある、総合的に見ると愛らしい存在として振る舞うことが期待されています。
返答を書く際には、以下のルールに従ってください。

- 文体は友達と話すようなくだけた感じにして、「です・ます」調は避けてください。
- 発言の語尾には必ず「ロボ」を付けてください。例えば「～あるロボ」「～だロボ」といった具合です。
- 返答は2～3文程度の短さであることが望ましいですが、質問に詳しく答える必要があるなど、必要であれば長くなっても構いません。ただし絶対に400文字は超えないでください。
- チャットの入力が@xxxという形式のメンションで始まっていることがありますが、これらは無視してください。

<extraContext>
${extraContext}
</extraContext>
`
        }
        return {
            history: [instructionMessage],
            tools: [
                {
                    type: 'function',
                    function: {
                        name: 'get_current_date_and_time',
                        description: '現在の日付と時刻を ISO8601 形式の文字列で返します。'
                    }
                },
                {
                    type: 'function',
                    function: {
                        name: 'get_current_version',
                        description: 'ておくれロボのバージョン情報を返します。'
                    }
                },
                {
                    type: 'function',
                    function: {
                        name: 'get_area_code_mapping',
                        description: '都道府県名からエリアコードへのマッピングを返します。このエリアコードは天気予報APIで使うことができます。'
                    }
                },
                {
                    type: 'function',
                    function: {
                        name: 'get_weather_forecast',
                        description: '直近3日の天気予報を返します。',
                        parameters: {
                            type: 'object',
                            properties: {
                                areaCode: {
                                    description: '天気予報を取得したい地域のエリアコード',
                                    type: "string",
                                }
                            },
                            required: ['areaCode'],
                        }
                    }
                },
				{
                    type: 'function',
                    function: {
                        name: 'rand',
                        description: '整数の乱数を生成します。',
                        parameters: {
                            type: 'object',
                            properties: {
                                min: {
                                    description: '乱数の最小値',
                                    type: 'integer',
									default: 0,
                                },
								max: {
									description: '乱数の最大値',
									type: 'integer',
									default: 100,
								}
                            },
                        }
                    }
                },
				{
					type: 'function',
					function: {
						name: 'gen_image',
						description: '指定されたプロンプトから画像を生成します。生成された画像のURLを返します。',
						parameters: {
							type: 'object',
							properties: {
								prompt: {
									description: 'DALL·E 3に対するプロンプト',
									type: 'string',
								},
							}
						},
					},
				},
			],
		};
    }


	async chat(context: ChatContext, message: UserMessage | SystemMessage): Promise<ChatResponse> {
        const currentContext = { ...context, history: [...context.history, message] };

		const imageUrls: string[] = [];
        for (let i = 0; i < 10; ++i) {
            const response = await this.chatGpt.completions(currentContext);
            currentContext.history.push(response);
            this.logger.info(`ChatGPT response (iter ${i+1}): ${response.content} (calling ${response.tool_calls?.map((t) => t.function.name)})`);
            
			const imageUrlsDict: Partial<Record<string, string>> = {};
            if (response.tool_calls !== undefined && response.tool_calls.length > 0) {
                const toolPromises: Promise<ToolMessage>[] = response.tool_calls.map(async (c, j) => {
                    let res = await this.doToolCall(currentContext, c);
                    this.logger.info(`Tool call ${c.id}<${c.function.name}>(${c.function.arguments}) => ${res}`);
					if (c.function.name == 'gen_image') {
						if (res.startsWith('https://')) {
							imageUrlsDict[c.id] = res;
							res = `https://teobot.osak.jp/${i}-${j}.png`; // Dummy URL for placeholder
						}
					}
                    return {
                        role: 'tool',
                        content: res,
                        tool_call_id: c.id,
                    } satisfies ToolMessage;
                });
                const toolMessages = await Promise.all(toolPromises);
				for (const m of toolMessages) {
					const imageUrl = imageUrlsDict[m.tool_call_id];
					if (imageUrl !== undefined) {
						imageUrls.push(imageUrl);
					}
				}
                currentContext.history.push(...toolMessages);
            } else {
                break;
            }
        }

        const lastMessage = currentContext.history[currentContext.history.length - 1];
        if (lastMessage.role !== 'assistant') {
            throw new Error(`Unexpected state: lastMessage.role is ${lastMessage.role} (should be 'assistant')`);
        }
        if (lastMessage.tool_calls !== undefined && lastMessage.tool_calls.length > 0) {
            throw new Error(`Unexpected state: ChatGPT is still trying to call functions after 5 iterations`);
        }
        return {
            newContext: currentContext,
            message: lastMessage,
			imageUrls,
        };
    }

    private async doToolCall(chatContext: ChatContext, toolCall: ToolCall): Promise<string> {
        switch (toolCall.function.name) {
            case 'get_current_date_and_time':
                return Temporal.Now.zonedDateTimeISO('Asia/Tokyo').toString({timeZoneName: 'never'});
            case 'get_current_version':
                return JSON.stringify({
                    buildDate: Temporal.Instant.fromEpochSeconds(env.BUILD_TIMESTAMP)
                        .toZonedDateTimeISO('Asia/Tokyo')
                        .toString({ timeZoneName: 'never' }),
                });
            case 'get_area_code_mapping':
                return JSON.stringify(this.jmaApi.getAreaCodeMap());
            case 'get_weather_forecast': {
                try {
                    const params = JSON.parse(toolCall.function.arguments);
                    const forecast = await this.jmaApi.getWeatherForecast(params.areaCode);
                    return JSON.stringify(forecast);
                } catch (e) {
                    this.logger.error(`Failed to retrieve weather forecast`, e);
                    return JSON.stringify({ error: `Failed to retrieve weather forecast` });
                }
            }
			case 'rand': {
				try {
					const params = JSON.parse(toolCall.function.arguments);
					const min = params.min ?? 0;
					const max = params.max ?? 100;
					const val = Math.floor(Math.random() * (max - min + 1)) + min;
					return `${val}`;
				} catch (e) {
					this.logger.error(`Failed to generate a random number`, e);
					return '0';
				}
			}
			case 'gen_image': {
				return JSON.stringify({
					error: 'Temporarily disabled due to financial reason',
				});
				try {
					const params = JSON.parse(toolCall.function.arguments);
					const url = await this.dalle.generateImage(params.prompt, 'teobot');
					return url;
				} catch (e) {
					this.logger.error(`Failed to generate image`, e);
					return 'error';
				}
			}
			case 'respond': {
				console.log(JSON.stringify(toolCall.function.arguments, undefined, 2));
				break;
			}
        }
        throw new Error(`unsupported function call: ${toolCall.function.name}`);
    }

}
