import * as dotenv from 'dotenv';
dotenv.config();

import * as readline from 'readline/promises';
import * as GlobalContext from '../globalContext';
import { TeobotService } from '../service/teobotService';
import { JmaApi } from '../api/jma';
import { DallE } from '../api/dalle';

async function main() {
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
    });
    const chatGPT = GlobalContext.chatGPT;
	const teobotService = new TeobotService(chatGPT, new JmaApi(), new DallE(GlobalContext.env.CHAT_GPT_API_KEY));

	let context = teobotService.newChatContext();
    while (true) {
        const line = await rl.question('> ');
        const response = await teobotService.chat(context, { role: 'user', content: line });
        console.log(`>> ${response.message.content}`);
        context = response.newContext;
    }
}

main();
