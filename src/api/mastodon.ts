import { Logger } from "../logging";
import { queryString } from "../util";

export interface Account {
    id: string;
    username: string; // e.g. osa_k
    acct: string; // e.g. osa_k (for local), osa_k@social.mikutter.hachune.net (for remote)
    display_name: string;
}

export interface Status {
    id: string;
    url: string;
    in_reply_to_id: string;
    in_reply_to_account_id: string;
    content: string;
    account: Account;
}

export type NotificationType = 'mention' | 'status' | 'reblog' | 'follow' | 'follow_request' | 'favourite' | 'poll' | 'update';

export interface Notification {
    id: string;
    type: NotificationType;
    account: Account;
    status?: Status;
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
        const accountInfo = await this.api<Account>('/api/v1/accounts/verify_credentials');
        return accountInfo;
    }

    async getStatus(id: string): Promise<Status> {
        return await this.api<Status>(`/api/v1/statuses/${id}`);
    }

    async getReplyTree(id: string): Promise<Context> {
        return await this.api<Context>(`/api/v1/statuses/${id}/context`);
    }

    async postStatus(content: string, replyToId?: string): Promise<void> {
        const payload = {
            status: content,
            in_reply_to_id: replyToId,
        };
        await this.api<void>(`/api/v1/statuses`, 'POST', payload);
    }

    async getAllNotifications(types: NotificationType[] = [], sinceId?: string): Promise<Notification[]> {
        const params = { since_id: sinceId, types };
        this.logger.info(queryString(params));
        return await this.api<Notification[]>(`/api/v1/notifications${queryString(params)}`);
    }

    private async api<T>(path: string, method: 'GET' | 'POST' = 'GET', body?: object): Promise<T> {
        const response = await fetch(`${this.baseUrl}${path}`, {
            headers: {
                'Authorization': `Bearer ${this.accessToken}`,
                'Content-Type': 'application/json',
            },
            method,
            body: body && JSON.stringify(body),
        });
        if (response.status != 200) {
            const errorMessage = await response.text();
            throw new Error(`Failed to call ${path}: ${errorMessage}`);
        }
        return await response.json() as T
    }
}
