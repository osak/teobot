import { Logger } from "../logging";
import { JsonApi } from "./jsonApi";

interface Image {
	b64_json?: string;
	url?: string;
	revised_prompt?: string;
}

interface GenerationsRequest {
	prompt: string;
	model: 'dall-e-2' | 'dall-e-3';
	n: 1;
	quality: 'hd' | 'standard';
	response_format: 'url';
	size: '1024x1024' | '1792x1024' | '1024x1792';
	style: 'vivid' | 'natural';
	user: string;
}

interface GenerationsResponse {
	created: number;
	data: Image[];
}

export class DallE {
	private readonly logger = Logger.createLogger('DallE');
	private readonly jsonApi: JsonApi;

	constructor(readonly apiKey: string) {
		this.jsonApi = new JsonApi(
			'https://api.openai.com/v1/images',
			{
				headers: () => ({
					'Authorization': `Bearer ${this.apiKey}`,
					'Content-Type': 'application/json',
				})
			}
		);
	}

	async generateImage(prompt: string, user: string): Promise<string> {
		const request: GenerationsRequest = {
			prompt,
			model: 'dall-e-3',
			n: 1,
			quality: 'standard',
			response_format: 'url',
			size: '1024x1024',
			style: 'vivid',
			user,
		};
		const response = await this.jsonApi.post<GenerationsResponse, GenerationsRequest>('/generations', request);
		this.logger.info(JSON.stringify(response));
		if (response.data.length == 0) {
			throw new Error(`Image generation API returned an empty response`);
		}
		const img = response.data[0];
		if (img.url === undefined) {
			this.logger.info(JSON.stringify(img));
			throw new Error(`Image generation API didn't return url`);
		}
		return img.url;
	}
}
