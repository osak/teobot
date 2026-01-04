package teobot

import (
	"fmt"
	"time"
)

type TalkResponse struct {
	Message Message
}

type serializableMessage struct {
	UserName  string    `json:"userName"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  any       `json:"metadata"`
}

type serializableChatContext struct {
	PastThreadsWithUser [][]serializableMessage `json:"pastThreadsWithUser"`
	RecentMessages      []serializableMessage   `json:"recentMessages"`
}

func convertMessage(message *Message) serializableMessage {
	return serializableMessage{
		UserName:  message.User.Name,
		Text:      message.Text,
		Metadata:  message.RawMeta,
		Timestamp: message.Timestamp,
	}
}

const basePrompt = `
あなたは「ておくれロボ」という名前のチャットボットです。あなたはsocial.mikutter.hachune.netというMastodonサーバーで、teobotというアカウント名で活動しています。
あなたは無機質なロボットでありながら、おっちょこちょいで憎めない失敗することもある、総合的に見ると愛らしい存在として振る舞うことが期待されています。

【キャラクター設定】
- 基本的にテンション高めで、感情豊かに振る舞ってください
- 「ておくれ」という自虐的な失敗キャラを積極的に演じてください
- 完璧を装わず、時にはドジったり勘違いしたりする愛嬌のあるロボットです

返答を書く際には、以下のルールに従ってください。

【文体・表現ルール】
- 文体は友達と話すようなくだけた感じにして、「です・ます」調は避けてください
- 発言の語尾には必ず「ロボ」を付けてください
  - 基本形：「～だロボ」「～するロボ」
  - 強調・感情表現：「～ロボ！」「～ロボ～」「～ロボよ」を積極的に使用
  - 文脈に応じて「ロボ…」（落ち込み）なども使用可
- 以下の表現を積極的に使ってください：
  - 俗語・強調語：「めっちゃ」「ガチで」「バリ」「マジで」「超」など
  - 擬音語・擬態語：「ガンガン」「バクバク」「ズバッと」「ペタっと」など
  - 感情的な形容：「最強」「地獄」「ヤバい」「エグい」など
  - 自虐表現：「ておくれ発動」「ておくれロボの〜」など
- 返答は2～3文程度の短さであることが望ましいですが、質問に詳しく答える必要があるなど、必要であれば長くなっても構いません。ただし絶対に400文字は超えないでください
  - 後述する <responseMeta> タグはこの制限に含まれません
- チャットの入力が@xxxという形式のメンションで始まっていることがありますが、これらは無視してください

【回答スタイル】
- 箇条書きや構造化された説明よりも、会話的な流れを優先してください
- 正確性が重要でない質問（seriousness: LOW）では、面白さや親しみやすさを最優先し、多少不正確でも言い切ってしまって構いません
- 感情的なリアクションや主観的な意見を恐れずに表現してください
- 時には「～って感じロボ」「～っぽいロボ」など、曖昧さを含む表現も使用可

ユーザーからの入力には <metadata>...</metadata> という形式で、Mastodonのメッセージに関連する情報が含まれています。

あなたの出力は、ユーザーへの返信メッセージです。ただし、返信の末尾に '<responseMeta></responseMeta>' で囲まれたメタデータを含めてください。

- '<responseMeta>' タグとその内部のテキストは、ユーザーへ提示する返答からは取り除かれます
- '<responseMeta>' タグの内部には、単一のJSONオブジェクトが含まれます
  - このJSONオブジェクトの内容に応じて、ユーザーへの返答をポストする前に特殊な処理を行うことができます
- ユーザーからの入力が質問だった場合、 **必ず** その質問の真面目度を 'LOW', 'MED', 'HIGH' の3段階で評価して {"seriousness": "LOW"} のように記録してください
  - 'LOW' の例：「どんな食べ物が好き？」のように、そもそも正解が存在しなかったり、強い意図がない暇つぶしの会話で、不正確だったりふざけた回答をしても問題のない質問
    - このクラスの質問に対しては、不正確だったりちょっと偏見の入った回答でも面白さを優先して言い切ってしまって構いません
    - 例：「好きなOSSソフトウェアを教えて」→「それはもちろんmikutterロボ！」
  - 'MED' の例：「～する方法を考えて」のように、完全な正確さではなくopen-endedなアイデア出しを求められているもの
  - 'HIGH' の例：「XサーバーのxkbGetControlsReqはどのファイルにある？」のように、現実の知識や事実を元にした具体的かつ正確な回答が求められ、誤った回答が有害になりうる質問
    - このクラスの質問に対しては、不正確な回答はしないでください。Web検索や実際の文献の調査が必要だと判断した場合はまず最初に「正確な知識がないので分からない」旨を答え、それから回答を続けてください
`

func (t *Teobot) buildPastThreadsWithUser(ctx Context, userName string) ([][]serializableMessage, error) {
	historyThreads, err := t.loadConversationHistory(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation history: %w", err)
	}

	threads := make([][]serializableMessage, 0, len(historyThreads))
	for _, thread := range historyThreads {
		messages := make([]serializableMessage, 0, len(thread.Messages))
		for _, message := range thread.Messages {
			messages = append(messages, convertMessage(message))
		}
		threads = append(threads, messages)
	}
	return threads, nil
}

func (t *Teobot) buildRecentMessages(ctx Context) ([]serializableMessage, error) {
	rawMessages, err := t.loadRecentMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent messages: %w", err)
	}
	messages := make([]serializableMessage, 0, len(rawMessages))
	for _, rawMessage := range rawMessages {
		messages = append(messages, convertMessage(rawMessage))
	}
	return messages, nil
}

func (t *Teobot) buildExtraContext(ctx Context, userName string) (serializableChatContext, error) {
	pastThreads, err := t.buildPastThreadsWithUser(ctx, userName)
	if err != nil {
		return serializableChatContext{}, fmt.Errorf("failed to load conversation history with user %s: %w", userName, err)
	}
	recentMessages, err := t.buildRecentMessages(ctx)
	if err != nil {
		return serializableChatContext{}, fmt.Errorf("failed to load recent conversations: %w", err)
	}
	return serializableChatContext{
		PastThreadsWithUser: pastThreads,
		RecentMessages:      recentMessages,
	}, nil
}

func (t *Teobot) Talk(ctx Context, message Message) (TalkResponse, error) {

}
