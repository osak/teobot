package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/perimeterx/marshmallow"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

type AccountTag struct {
	Id   string `json:"id"`
	Acct string `json:"acct"`
}

type MessageTag struct {
	Id                 string     `json:"id"`
	Url                string     `json:"url"`
	InReplyToId        string     `json:"in_reply_to_id"`
	InReplyToAccountId string     `json:"in_reply_to_account_id"`
	Account            AccountTag `json:"account"`
}

type Message struct {
	tag MessageTag
	raw map[string]interface{}
}

type Envelope struct {
	Messages []map[string]interface{} `json:"messages"`
}

func loadMessages(path string) ([]*Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var envelope Envelope
	if _, err := marshmallow.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	logger.Info("Loaded envelope", "path", path, "messages", len(envelope.Messages))

	var messages []*Message
	for _, message := range envelope.Messages {
		var tag MessageTag
		_, err := marshmallow.UnmarshalFromJSONMap(message, &tag)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &Message{
			tag: tag,
			raw: message,
		})
	}
	return messages, nil
}

func loadMessagesFromDir(dirPath string) ([]*Message, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var res []*Message
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		path := dirPath + "/" + file.Name()
		messages, err := loadMessages(path)
		if err != nil {
			logger.Error("Failed to load message", "path", path, "error", err)
			continue
		}
		res = append(res, messages...)
	}
	return res, nil
}

func uniqueMessages(messages []*Message) map[string]*Message {
	unique := make(map[string]*Message)
	for _, message := range messages {
		unique[message.tag.Id] = message
	}
	return unique
}

type UnionFind struct {
	parents map[string]string
}

func NewUnionFind() *UnionFind {
	return &UnionFind{
		parents: make(map[string]string),
	}
}

func (uf *UnionFind) Find(x string) string {
	if parent, exists := uf.parents[x]; exists {
		if parent != x {
			parent = uf.Find(parent)
			uf.parents[x] = parent
		}
		return parent
	}
	uf.parents[x] = x
	return x
}

type Thread struct {
	RootStatusId string
	Participants []*AccountTag
	Messages     []*Message
}

func saveThread(destDir string, thread *Thread) error {
	if len(thread.Messages) == 0 {
		return nil
	}

	messages := make([]map[string]interface{}, len(thread.Messages))
	for i, message := range thread.Messages {
		messages[i] = message.raw
	}

	envelope := map[string]interface{}{
		"participants": thread.Participants,
		"messages":     messages,
	}

	// Save the envelope for each participant
	for _, participant := range thread.Participants {
		// Create the directory if it doesn't exist
		outDir := fmt.Sprintf("%s/%s", destDir, participant.Acct)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			logger.Error("Failed to create directory", "error", err)
		}
		// Create the file path
		filePath := fmt.Sprintf("%s/%s.json", outDir, thread.RootStatusId)
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		enc := json.NewEncoder(file)
		if err := enc.Encode(envelope); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	messages, err := loadMessagesFromDir(os.Args[1])
	if err != nil {
		logger.Error("Failed to load messages", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d messages\n", len(messages))

	unique := uniqueMessages(messages)
	fmt.Printf("Unique messages: %d\n", len(unique))

	uf := NewUnionFind()
	for _, message := range messages {
		if message.tag.InReplyToId != "" {
			uf.parents[message.tag.Id] = message.tag.InReplyToId
		} else {
			uf.parents[message.tag.Id] = message.tag.Id
		}
	}

	threads := make(map[string]*Thread)
	for _, message := range messages {
		root := uf.Find(message.tag.Id)
		if _, exists := threads[root]; !exists {
			threads[root] = &Thread{
				RootStatusId: root,
				Messages:     []*Message{},
			}
		}
		threads[root].Messages = append(threads[root].Messages, message)
	}
	for _, thread := range threads {
		slices.SortFunc(thread.Messages, func(i, j *Message) int {
			return cmp.Compare(i.tag.Id, j.tag.Id)
		})
		participants := make(map[string]*AccountTag)
		for _, message := range thread.Messages {
			participants[message.tag.Account.Id] = &message.tag.Account
		}
		thread.Participants = make([]*AccountTag, 0, len(participants))
		for _, participant := range participants {
			thread.Participants = append(thread.Participants, participant)
		}
	}

	fmt.Printf("Threads: %d\n", len(threads))

	for _, thread := range threads {
		if err := saveThread(os.Args[2], thread); err != nil {
			logger.Error("Failed to save thread", "error", err)
		}
	}
}
