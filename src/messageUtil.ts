import { Status } from "./api/mastodon";

export function normalizeStatusContent(status: Status): string {
	return stripHeadMentions(stripHtmlTags(status.content));
}

function stripHeadMentions(text: string): string {
	return text.replaceAll(/^\s*(@[a-zA-Z0-9_]+\s*)+/g, '');
}

export function stripHtmlTags(text: string): string {
    return text.replaceAll(/<br \/>/g, " ").replaceAll(/<[^>]+>/g, '');
}
