import { Logger } from "../logging";

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
	imageUrls: string[];
}

export class ChatGPT {
    private readonly logger = Logger.createLogger('chatgpt');

    constructor(readonly apiKey: string) {}

    async completions(chatContext: ChatContext): Promise<AssistantMessage> {
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
