package main

import (
	"github.com/sashabaranov/go-openai"
	"os"
)

func loadSystemMessages() map[string]openai.ChatCompletionMessage {
	files := map[string]string{
		"ask":      "prompts/ask.txt",
		"lesson":   "prompts/lesson.txt",
		"feedback": "prompts/feedback.txt",
		"test":     "prompts/test.txt",
	}

	messages := make(map[string]openai.ChatCompletionMessage)

	for key, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			panic("Failed to read " + path + ": " + err.Error())
		}
		messages[key] = openai.ChatCompletionMessage{
			Role:    "system",
			Content: string(data),
		}
	}

	return messages
}
