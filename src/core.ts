import { ChatContext, ChatGPT, Message, UserMessage } from "./chatgpt";
import { GlobalContext } from "./globalContext";
import { Logger } from "./logging";

const logger = Logger.createLogger('core');

export async function doChat(context: ChatContext, message: string): Promise<ChatContext> {
    const newMessage: UserMessage = {
        role: 'user',
        content: message,
    };
    const response = await GlobalContext.chatGPT.chat(context, newMessage);
    return response.newContext;
}