import { Status } from "./api/mastodon";

export function normalizeStatusContent(status: Status): string {
	return stripHeadMentions(stripHtml(status.content));
}

function stripHeadMentions(text: string): string {
	return text.replaceAll(/^\s*(@[a-zA-Z0-9_]+\s*)+/g, '');
}

function stripHtml(text: string): string {
    return text.replaceAll(/<br \/>/g, " ").replaceAll(/<[^>]+>/g, '');
}
