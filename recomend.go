package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

type RecomendRequest struct {
	Question string `json:"question"`
}

type RecomendResponse struct {
	Result  string                      `json:"result"`
	Courses map[string][]map[string]any `json:"courses"`
}

func recomendSimpleHandler(rb *RequestBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RecomendRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Получаем рекомендацию от AI
		result, err := rb.AddRequest("yourlanguage", req.Question, "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI error"})
			return
		}

		// Извлекаем языки из результата (по строкам после "languages:")
		lines := strings.Split(result, "\n")
		capture := false
		var langs []string
		for _, line := range lines {
			if strings.TrimSpace(line) == "languages:" {
				capture = true
				continue
			}
			if capture && strings.TrimSpace(line) != "" {
				langs = append(langs, strings.TrimSpace(line))
			}
		}

		// Получаем список всех курсов
		resp, err := http.Get("https://quant.up.railway.app/courses/")
		if err != nil || resp.StatusCode != 200 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch courses"})
			return
		}
		defer resp.Body.Close()

		var allCourses []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&allCourses); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid courses JSON"})
			return
		}

		filtered := make(map[string][]map[string]any)
		for _, lang := range langs {
			for _, course := range allCourses {
				if langVal, ok := course["language"].(string); ok && strings.EqualFold(langVal, lang) {
					filtered[lang] = append(filtered[lang], map[string]any{
						"id":       course["id"],
						"language": course["language"],
					})
				}
			}
			if _, found := filtered[lang]; !found {
				filtered[lang] = nil
			}
		}

		c.JSON(http.StatusOK, RecomendResponse{
			Result:  result,
			Courses: filtered,
		})
	}
}
