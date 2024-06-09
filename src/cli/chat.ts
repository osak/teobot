import * as dotenv from 'dotenv';
dotenv.config();

import * as readline from 'readline/promises';
import * as GlobalContext from '../globalContext';
import { TeobotService } from '../service/teobotService';
import { JmaApi } from '../api/jma';
import { DallE } from '../api/dalle';
import { TextSplitService } from '../service/textSplitService';

async function main() {
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
    });
    const chatGPT = GlobalContext.chatGPT;
	const teobotService = new TeobotService(chatGPT, new JmaApi(), new DallE(GlobalContext.env.CHAT_GPT_API_KEY));
	const textSplitService = new TextSplitService(chatGPT);

	let context = teobotService.newChatContext();
    while (true) {
        const line = await rl.question('> ');
		const match = line.match(/^split (\d+) (.*)$/);
		if (match) {
			const response = await textSplitService.splitText(match[2], parseInt(match[1]));
			if (response.type == 'ok') {
				console.log(`>> ${JSON.stringify(response.value)}`);
			} else {
				console.log(`ERROR: ${response.value}`);
			}
		} else {
			const response = await teobotService.chat(context, { role: 'user', content: line });
			let content = response.message.content;
			if (content && content.length > 400) {
				const partsRes = await textSplitService.splitText(content, Math.ceil(content.length / 400));
				if (partsRes.type == 'ok') {
					content = partsRes.value.map((p, i) => `----${i}-----\n${p}`).join("\n");
				} else {
					console.log('Failed to split a long message. Showing raw');
				}
			}

			console.log(`>> ${content}`);
			context = response.newContext;
		}
    }
}

main();
