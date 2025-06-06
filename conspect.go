package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
)

type ConspectRequest struct {
	Messages   []openai.ChatCompletionMessage `json:"messages"`
	Topic      string                         `json:"topic"`
	Language   string                         `json:"language"`
	RulesStyle string                         `json:"rules_style"`
}

func conspectHandler(client *openai.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ConspectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		system := openai.ChatCompletionMessage{
			Role: "system",
			Content: "You are a helpful assistant that generates well-structured educational content " +
				"in the style requested by the user.",
		}

		// Вставим короткое пояснение в user-запрос в самом начале
		formattedIntro := openai.ChatCompletionMessage{
			Role: "user",
			Content: "Topic: " + req.Topic + "\n" +
				"Language: " + req.Language + "\n" +
				"Style: " + req.RulesStyle + "\n" +
				"Now generate the text according to this request and keep previous messages in mind.",
		}

		// Сформируем messages: system + intro + все предыдущие
		messages := []openai.ChatCompletionMessage{system, formattedIntro}
		messages = append(messages, req.Messages...)

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini",
			Messages: messages,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"title": req.Topic,
			"text":  resp.Choices[0].Message.Content,
		})
	}
}
