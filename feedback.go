package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
)

func feedbackHandler(rb *RequestBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := rb.AddRequest("feedback", req.Question, req.Input, req.Language)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		text, code := splitTextAndCode(result)

		c.JSON(http.StatusOK, gin.H{
			"text": text,
			"code": code,
		})
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
