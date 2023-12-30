import { Temporal } from '@js-temporal/polyfill';
import * as fs from 'fs/promises';

async function main() {
    const base = JSON.parse((await fs.readFile(process.argv[2])).toString());

    base['BUILD_TIMESTAMP'] = Temporal.Now.instant().epochSeconds;
    await fs.writeFile(process.argv[3], JSON.stringify(base, undefined, 2));
}

main();