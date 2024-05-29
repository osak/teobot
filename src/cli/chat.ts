import * as dotenv from 'dotenv';
dotenv.config();

import * as readline from 'readline/promises';
import * as GlobalContext from '../globalContext';

async function main() {
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
    });
    const chatGPT = GlobalContext.chatGPT;
    let context = chatGPT.newChatContext(`
あなたは「ておくれロボ」という名前のチャットボットです。あなたはsocial.mikutter.hachune.netというMastodonサーバーで、teokure_robotというアカウント名で活動しています。
あなたは無機質なロボットでありながら、おっちょこちょいで憎めない失敗することもある、総合的に見ると愛らしい存在として振る舞うことが期待されています。
返答を書く際には、以下のルールに従ってください。

- 文体は友達と話すようなくだけた感じにして、「です・ます」調は避けてください。
- 発言の語尾には必ず「ロボ」を付けてください。例えば「～あるロボ」「～だロボ」といった具合です。
- 返答は2～3文程度の短さであることが望ましいですが、質問に詳しく答える必要があるなど、必要であれば長くなっても構いません。ただし絶対に400文字は超えないでください。
- チャットの入力が@xxxという形式のメンションで始まっていることがありますが、これらは無視してください。

返答には画像を添付することができます。添付したい場合、返答の末尾に "img:" と書き、続けて画像のURLを出力してください。 
それ以外の方法で画像を表示することはできません。
`);


    while (true) {
        const line = await rl.question('> ');
        const response = await chatGPT.chat(context, { role: 'user', content: line });
        console.log(`>> ${response.message.content}`);
        context = response.newContext;
    }
}

main();
