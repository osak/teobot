import { Temporal } from "@js-temporal/polyfill";
import { Logger } from "../logging";
import { env } from '../globalContext';
import { JmaApi } from "./jma";
import { DallE } from "./dalle";

type Role = 'system' | 'user' | 'assistant' | 'tool';

export interface FunctionDefinition {
    name: string;
    description: string;
    parameters?: object;
}

export interface FunctionCallDescriptor {
    name: string;
    arguments: string; // String-serialized JSON, may be invalid due to hallucination
}

export interface Tool {
    type: 'function';
    function: FunctionDefinition;
}

export interface ToolCall {
    id: string;
    type: 'function';
    function: FunctionCallDescriptor;
}

export interface SystemMessage {
    role: Extract<Role, 'system'>;
    content: string;
    name?: string;
}

export interface UserMessage {
    role: Extract<Role, 'user'>;
    content: string;
    name?: string;
}

export interface AssistantMessage {
    role: Extract<Role, 'assistant'>;
    content?: string | null;
    name?: string;
    tool_calls?: ToolCall[];
}

export interface ToolMessage {
    role: Extract<Role, 'tool'>;
    content: string;
    tool_call_id: string;
}

export type Message = UserMessage | SystemMessage | UserMessage | AssistantMessage | ToolMessage;

export type FinishReason = 'stop' | 'length' | 'content_filter' | 'tool_calls';

export interface ChatCompletionChoice {
    index: number;
    message: Message;
    finish_reason: FinishReason;
}

export interface Usage {
    completion_tokens: number;
    prompt_tokens: number;
    total_tokens: number; // completion_tokens + prompt_tokens
}

export interface ChatCompletion {
    id: string;
    choices: ChatCompletionChoice[];
    created: number; // Unix timestamp in seconds
    model: string;
    system_fingerprint: string;
    object: 'chat.completion';
    usage: Usage;
}

export interface ChatContext {
    history: Message[];
    tools: Tool[];
	tool_choice?: {
		type: 'function',
		function: {
			name: string,
		}
	};
}

export interface ChatRequest {
    model: string;
    messages: Message[];
    tools: Tool[];
}

export interface ChatResponse {
    newContext: ChatContext;
    message: Message;
}

export class ChatGPT {
    private readonly logger = Logger.createLogger('chatgpt');
    private readonly jmaApi: JmaApi;
	private readonly dalle: DallE;

    constructor(readonly apiKey: string) {
        this.jmaApi = new JmaApi();
		this.dalle = new DallE(apiKey);
    }

    newChatContext(instruction: string): ChatContext {
        const instructionMessage: SystemMessage = {
            role: 'system',
            content: instruction,
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

        for (let i = 0; i < 10; ++i) {
            const response = await this.doChat(currentContext);
            currentContext.history.push(response);
            this.logger.info(`ChatGPT response (iter ${i+1}): ${response.content} (calling ${response.tool_calls?.map((t) => t.function.name)})`);
            
            if (response.tool_calls !== undefined && response.tool_calls.length > 0) {
                const toolPromises: Promise<ToolMessage>[] = response.tool_calls.map(async (c) => {
                    const res = await this.doToolCall(currentContext, c);
                    this.logger.info(`Tool call ${c.id}<${c.function.name}>(${c.function.arguments}) => ${res}`);
                    return {
                        role: 'tool',
                        content: res,
                        tool_call_id: c.id,
                    } satisfies ToolMessage;
                });
                const toolMessages = await Promise.all(toolPromises);
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
        };
    }

    private async doChat(chatContext: ChatContext): Promise<AssistantMessage> {
        const completion = await this.api<ChatCompletion, ChatRequest>('https://api.openai.com/v1/chat/completions', {
            model: 'gpt-4o',
            messages: chatContext.history,
            tools: chatContext.tools
        });
        if (completion.choices.length == 0) {
            throw new Error('ChatGPT returns empty response');
        }

        const response = completion.choices[0];
        if (response.message.role === 'assistant') {
            return response.message;
        } else {
            throw new Error(`ChatGPT returns non-assistant response: ${JSON.stringify(response)}`);
        }
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

    private async api<T, B = undefined>(url: string, body?: B): Promise<T> {
		const controller = new AbortController();
		const timeout = setTimeout(() => controller.abort(), 90 * 1000);

        const response = await fetch(url, {
            headers: {
                'Authorization': `Bearer ${this.apiKey}`,
                'Content-Type': 'application/json',
            },
            body: body && JSON.stringify(body),
            method: 'POST',
			signal: controller.signal,
        });
		clearTimeout(timeout);

        if (response.status != 200) {
            const text = await response.text();
            throw new Error(text);
        }
        return await response.json() as T;
    }
}
