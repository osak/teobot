import { Status } from "../api/mastodon";
import { Logger } from "../logging";
import * as fs from 'fs';

export class HistoryService {
    private readonly logger = new Logger('historyService');

    constructor(
        private readonly dataDir: string,
    ) {}

    async getHistory(acct: string, limit: number): Promise<Status[][]> {
        this.logger.info(`Loading history for ${acct} from ${this.dataDir}`);

        const historyDirPath = `${this.dataDir}/threads/${acct}`;
        if (!fs.existsSync(historyDirPath)) {
            return [];
        }

        const files = fs.readdirSync(historyDirPath);
        files.sort().reverse();

        const history: Status[][] = [];
        files.slice(0, limit).forEach(file => {
            const filePath = `${historyDirPath}/${file}`;
            const data = fs.readFileSync(filePath, 'utf-8');
            const parsedData = JSON.parse(data);
            history.push(parsedData.messages);
        });

        return history;
    }
}