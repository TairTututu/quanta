package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"regexp"
)

var featurePrompts map[string]string

func init() {
	data, err := os.ReadFile("features.yaml")
	if err != nil {
		panic("Cannot load features.yaml: " + err.Error())
	}
	var config struct {
		Features map[string]string `yaml:"features"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		panic("YAML parse error: " + err.Error())
	}
	featurePrompts = config.Features
}

func feedbackHandler(rb *RequestBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if prompt, ok := featurePrompts[req.Question]; ok {
			result, err := rb.client.CreateChatCompletion(c, openai.ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []openai.ChatCompletionMessage{
					{Role: "system", Content: prompt},
					{Role: "user", Content: req.Input},
				},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			text, code := splitTextAndCode(result.Choices[0].Message.Content)
			c.JSON(http.StatusOK, gin.H{"text": text, "code": code})
			return
		}

		result, err := rb.AddRequest("feedback", req.Question, req.Input, req.Language)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		text, code := splitTextAndCode(result)
		c.JSON(http.StatusOK, gin.H{"text": text, "code": code})
	}
}

func splitTextAndCode(s string) (string, string) {
	re := regexp.MustCompile("(?s)```(?:[a-z]+)?\\s*(.*?)\\s*```")
	matches := re.FindStringSubmatch(s)

	if len(matches) >= 2 {
		code := matches[1]
		text := re.ReplaceAllString(s, "")
		return text, code
	}
	return s, ""
}
