package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
)

var featurePrompts map[string]string

func loadFeaturePrompts(path string) {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read feature prompts: %v", err)
	}
	var data map[string]map[string]string
	if err := yaml.Unmarshal(file, &data); err != nil {
		log.Fatalf("failed to parse yaml: %v", err)
	}
	featurePrompts = data["features"]
}

type FeatureRequest struct {
	Input    string `json:"input"`
	Feature  string `json:"feature"`
	Language string `json:"language"`
}

func compilerFeatureHandler(client *openai.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req FeatureRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		prompt, ok := featurePrompts[req.Feature]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported feature"})
			return
		}

		messages := []openai.ChatCompletionMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: req.Input},
		}

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini",
			Messages: messages,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		text, code := splitTextAndCode(resp.Choices[0].Message.Content)
		c.JSON(http.StatusOK, gin.H{"text": text, "code": code})
	}
}
