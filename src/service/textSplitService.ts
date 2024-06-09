import { ChatContext, ChatGPT, Message, Tool, ToolCall, UserMessage } from "../api/chatgpt";
import { Logger } from "../logging";
import { Result, err, ok } from "../util";

export class TextSplitService {
	private readonly logger = new Logger('textSplitService');

	constructor(private readonly chatGpt: ChatGPT) {}

	async splitText(text: string, numParts: number): Promise<Result<string[], string>> {
		const history: Message[] = [
			{
				role: 'system',
				content: `これから入力される文章を、意味の区切りを考えて${numParts}個の部分に分割してください。それぞれの部分の長さは500文字以下に収めてください。分割結果はreport_result関数を呼び出すことで報告してください。元の文章を改編したり省略したりはせず、全文が必ずそのまま結果に含まれるように気をつけてください。`
			}
		];
		const tools: Tool[] = [
			{
				type: 'function',
				function: {
					name: 'report_result',
					description: '文章の分割結果を報告します',
					parameters: {
						type: 'object',
						properties: {
							parts: {
								description: '分割された部分からなる配列',
								type: 'array',
								items: { type: 'string' }
							}
						}
					}
				}
			}
		];
		const message: UserMessage = {
			role: 'user',
			content: text,
		};
		const context: ChatContext = {
			history: [...history, message],
			tools,
			tool_choice: {
				type: 'function',
				function: {
					name: 'report_result',
				}
			},
		};

		const response = await this.chatGpt.completions(context);
		if (response.tool_calls) {
			return this.doReportResult(response.tool_calls[0]);
		} else {
			return err('Failed to split');
		}
	}

	private doReportResult(call: ToolCall): Result<string[], string> {
		if (call.function.name !== 'report_result') {
			return err(`Unknown function call: ${call.function.name}`);
		}
		this.logger.info(call.function.arguments);
		const payload = JSON.parse(call.function.arguments);
		const parts = payload['parts'];
		if (!(parts instanceof Array)) {
			return err(`Invalid payload for report_result call: ${payload}`);
		}

		return ok(parts);
	}
}
