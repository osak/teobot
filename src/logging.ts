import { Temporal } from "@js-temporal/polyfill";
import { padRight } from "./util";

type LogLevel = 'error' | 'warn' | 'info';

export class Logger {
    static createLogger(name: string): Logger {
        return new Logger(name);
    }

    constructor(readonly name: string) {}

    info(message: string) {
        this.log('info', message);
    }

    warn(message: string) {
        this.log('warn', message);
    }

    error(message: string, cause?: unknown) {
        let fullMessage = message;
        if (cause instanceof Error) {
            fullMessage += `\n${cause.message}`;
            if (cause.stack) {
                fullMessage += `\nStacktrace:\n${cause.stack}`;
            }
        }
        this.log('error', fullMessage);
    }

    log(level: LogLevel, message: string) {
        const label = padRight(this.levelToStr(level), 5);
        console.log(`[${Temporal.Now.zonedDateTimeISO('Asia/Tokyo').toString({timeZoneName: 'never'})}][${label}] ${message}`);
    }

    private levelToStr(level: LogLevel): string {
        switch (level) {
            case 'error': return 'ERROR';
            case 'warn': return 'WARN';
            case 'info': return 'INFO';
        }
    }
}
