package teobot

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/osak/teobot/internal/service/chatgpt"
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

// buildPastThreadsWithUser builds a collection of messages in recent conversations with the user.
func (t *Teobot) buildPastThreadsWithUser(ctx context.Context, userName string) ([][]serializableMessage, error) {
	historyThreads, err := t.loadConversationHistory(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation history: %w", err)
	}

	threads := make([][]serializableMessage, 0, len(historyThreads))
	for _, thread := range historyThreads {
		messages := make([]serializableMessage, 0, len(thread.Messages))
		for _, message := range thread.Messages {
			messages = append(messages, convertMessage(&message))
		}
		threads = append(threads, messages)
	}
	return threads, nil
}

// buildRecentMessages build a collection of messages in recent conversations.
func (t *Teobot) buildRecentMessages(ctx context.Context) ([]serializableMessage, error) {
	rawMessages, err := t.loadRecentMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent messages: %w", err)
	}
	messages := make([]serializableMessage, 0, len(rawMessages))
	for _, rawMessage := range rawMessages {
		messages = append(messages, convertMessage(&rawMessage))
	}
	return messages, nil
}

// buildExtraContext builds a context object to feed to the bot.
func (t *Teobot) buildExtraContext(ctx context.Context, userName string) (serializableChatContext, error) {
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

// buildSystemMessages builds ChatGPT system messages to instruct the bot about their role and give the context.
func (t *Teobot) buildSystemMessages(ctx context.Context, userName string) ([]chatgpt.Message, error) {
	extraContext, err := t.buildExtraContext(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("build extra context: %w", err)
	}
	extraContextJson, err := json.Marshal(extraContext, jsontext.EscapeForHTML(false))
	if err != nil {
		return nil, fmt.Errorf("marshal extra context: %w", err)
	}
	systemPromptMessage := chatgpt.Message{
		Role: "system",
		Content: []chatgpt.MessageContent{
			chatgpt.InputText{
				Text: basePrompt,
			},
		},
	}
	extraContextMessage := chatgpt.Message{
		Role: "system",
		Content: []chatgpt.MessageContent{
			chatgpt.InputText{
				Text: string(extraContextJson),
			},
		},
	}
	return []chatgpt.Message{systemPromptMessage, extraContextMessage}, nil
}

func (t *Teobot) buildCurrentThreadMessages(ctx context.Context, threadID uuid.UUID) ([]chatgpt.Message, error) {
	messages, err := t.repository.LoadMessagesInThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	cgMessages := make([]chatgpt.Message, len(messages))
	for i, message := range messages {
		if message.User.Name == "teobot" {
			cgMessages[i] = message.ToChatGptMessage(chatgpt.RoleAssistant)
		} else {
			cgMessages[i] = message.ToChatGptMessage(chatgpt.RoleUser)
		}
	}

	return cgMessages, nil
}

// Talk generates a bot response from the given context and the message to reply to.
func (t *Teobot) Talk(ctx context.Context, replyToMessageID uuid.UUID, message *Message) (*TalkResponse, error) {
	// Identify the current thread
	var threadID uuid.UUID
	if replyToMessageID == uuid.Nil {
		// The message is very beginning of a thread - create a new one.
		thread, err := t.queries.CreateChatgptThread(ctx, uuid.Must(uuid.NewV7()))
		if err != nil {
			return nil, fmt.Errorf("create a new thread: %w", err)
		}
		threadID = thread.ID
	} else {
		// The message is replying to an existing message. Identify which thread the message belongs to.
		var err error
		threadID, err = t.FindOngoingThreadIDByMessageID(ctx, replyToMessageID)
		if errors.Is(err, ErrNoThread) {
			threadID, err = t.ForkThread(ctx, replyToMessageID)
		}
		if err != nil {
			return nil, fmt.Errorf("find ongoing thread (messageID=%s): %w", replyToMessageID, err)
		}
	}
	slog.Debug(fmt.Sprintf("Thread ID: %s", threadID))

	systemMessages, err := t.buildSystemMessages(ctx, message.User.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to build system messages: %w", err)
	}
	threadMessages, err := t.buildCurrentThreadMessages(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("build current thread: %w", err)
	}

	inputMessages := make([]chatgpt.Input, 0, len(systemMessages)+len(threadMessages)+1)
	for _, systemMessage := range systemMessages {
		inputMessages = append(inputMessages, systemMessage)
	}
	for _, threadMessages := range threadMessages {
		inputMessages = append(inputMessages, threadMessages)
	}
	inputMessages = append(inputMessages, message.ToChatGptMessage(chatgpt.RoleUser))

	// Call ChatGPT to generate the response
	req := chatgpt.ResponsesRequest{
		Input: inputMessages,
		Model: "gpt-5",
		Reasoning: chatgpt.ReasoningEffort{
			Effort: "minimal",
		},
	}
	res, err := t.chatGpt.Responses(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ChatGPT: %w", err)
	}
	if res.Error != nil {
		return nil, fmt.Errorf("ChatGPT returned an error: %s", res.Error)
	}

	// Parse response
	resText := ""
	for _, output := range res.Output {
		if m, ok := output.(*chatgpt.Message); ok {
			for _, content := range m.Content {
				if t, ok := content.(*chatgpt.OutputText); ok {
					resText += t.Text + "\n"
				} else {
					slog.Info(fmt.Sprintf("Unprocessed response content: %#v", content))
				}
			}
		} else {
			slog.Info(fmt.Sprintf("Unprocessed response output: %#v", output))
		}
	}
	resMsg := Message{
		ID:           uuid.Must(uuid.NewV7()),
		Text:         resText,
		PrivacyLevel: message.PrivacyLevel,
		User:         t.user,
		Timestamp:    time.Now(),
	}

	// Save the original message and response
	if err = t.repository.SaveMessages(ctx, threadID, *message, resMsg); err != nil {
		return nil, fmt.Errorf("save messages: %w", err)
	}

	return &TalkResponse{Message: resMsg}, nil
}
