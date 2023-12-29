export function padRight(s: string, minWidth: number): string {
    const diff = minWidth - s.length;
    if (diff <= 0) {
        return s;
    }

    const pad = ' '.repeat(diff);
    return `${s}${pad}`;
}

export function queryString(params: { [key: string]: string | string[] | undefined }): string {
    const fragments = Object.entries(params).map((entry) => {
        const [k, v] = entry;
        if (v === undefined) {
            return null;
        }
        if (typeof v === 'object') {
            const arr = v as string[];
            if (arr.length > 0) {
                return arr.map((val) => `${k}[]=${val}`);
            } else {
                return null;
            }
        } else {
            return `${k}=${v}`;
        }
    }).flat().filter((f) => f !== null);

    const paramsString = fragments.join('&');
    if (paramsString !== '') {
        return `?${paramsString}`;
    } else {
        return '';
    }
}

export function stripHtmlTags(text: string): string {
    return text.replaceAll(/<br \/>/g, " ").replaceAll(/<[^>]+>/g, '');
}