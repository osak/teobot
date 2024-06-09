import { Logger } from "../logging";
import { queryString } from "../util";

export interface Account {
    id: string;
    username: string; // e.g. osa_k
    acct: string; // e.g. osa_k (for local), osa_k@social.mikutter.hachune.net (for remote)
    display_name: string;
}

export type Visibility = 'public' | 'unlisted' | 'private' | 'direct';

export interface Status {
    id: string;
    url: string;
    in_reply_to_id: string;
    in_reply_to_account_id: string;
    content: string;
    account: Account;
	visibility: Visibility;
}

export interface PostStatusOpt {
	replyToId?: string,
	mediaIds?: string[],
	visibility?: Visibility,
	sensitive?: boolean,
}

export type NotificationType = 'mention' | 'status' | 'reblog' | 'follow' | 'follow_request' | 'favourite' | 'poll' | 'update';

export interface Notification {
    id: string;
    type: NotificationType;
    account: Account;
    status?: Status;
}

export interface MediaAttachment {
	id: string;
	url?: string;
}

export interface MediaAttachmentWithStatus extends MediaAttachment {
	status: 'uploading' | 'uploaded' | 'error';
}

export interface Context {
    ancestors: Status[];
    descendants: Status[];
}

export class Mastodon {
    private readonly logger: Logger = Logger.createLogger('mastodon');

    constructor(
        private readonly baseUrl: string,
        private readonly clientKey: string,
        private readonly clientSecret: string,
        private readonly accessToken: string,
    ) {}

    async verifyCredentials(): Promise<Account> {
        const accountInfo = await this.defaultResponseHandler<Account>(await this.api('/api/v1/accounts/verify_credentials'));
        return accountInfo;
    }

    async getStatus(id: string): Promise<Status> {
        return this.defaultResponseHandler<Status>(await this.api(`/api/v1/statuses/${id}`));
    }

    async getReplyTree(id: string): Promise<Context> {
        return this.defaultResponseHandler<Context>(await this.api(`/api/v1/statuses/${id}/context`));
    }

    async postStatus(content: string, opt?: PostStatusOpt): Promise<Status> {
        const payload = {
            status: content,
            in_reply_to_id: opt?.replyToId,
			media_ids: opt?.mediaIds,
			visibility: opt?.visibility,
			sensitive: opt?.sensitive,
        };
        return await this.defaultResponseHandler<Status>(await this.api(`/api/v1/statuses`, 'POST', payload));
    }

    async getAllNotifications(types: NotificationType[] = [], sinceId?: string): Promise<Notification[]> {
        const params = { since_id: sinceId, types };
        this.logger.info(queryString(params));
        return this.defaultResponseHandler<Notification[]>(await this.api(`/api/v1/notifications${queryString(params)}`));
    }

	async uploadImage(buffer: Buffer): Promise<MediaAttachmentWithStatus> {
		const blob = new Blob([buffer]);
		const formData = new FormData();
		formData.append('file', blob);

		const response = await this.postFormData('/api/v2/media', formData);
		const body = await response.json() as MediaAttachment;
		if (response.status == 200) {
			return { ...body, status: 'uploaded' };
		} else if (response.status == 202) {
			return { ...body, status: 'uploading' };
		} else {
			return { ...body, status: 'error' };
		}
	}

	async getImage(id: string): Promise<MediaAttachmentWithStatus> {
		const response = await this.api(`/api/v1/media/${id}`);
		const body = await response.json() as MediaAttachment;
		if (response.status == 200) {
			return { ...body, status: 'uploaded' };
		} else if (response.status == 206) {
			return { ...body, status: 'uploading' };
		} else {
			return { ...body, status: 'error' };
		}
	}

    private async api(path: string, method: 'GET' | 'POST' = 'GET', body?: object): Promise<Response> {
        return fetch(`${this.baseUrl}${path}`, {
            headers: {
                'Authorization': `Bearer ${this.accessToken}`,
                'Content-Type': 'application/json',
            },
            method,
            body: body && JSON.stringify(body),
        });
	}

	private async postFormData(path: string, formData: FormData): Promise<Response> {
		return await fetch(`${this.baseUrl}${path}`, {
			headers: {
                'Authorization': `Bearer ${this.accessToken}`,
			},
			method: 'POST',
			body: formData,
		});
	}

	private async defaultResponseHandler<T>(response: Response): Promise<T> {
		if (response.status != 200) {
            const errorMessage = await response.text();
            throw new Error(`Failed to call ${response.url}: ${errorMessage}`);
        }
        return await response.json() as T;
	}
}
