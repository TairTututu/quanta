package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var API_KEY = os.Getenv("OPENAI_API_KEY")

const batchWindowMs = 100

var systemMessages = map[string]openai.ChatCompletionMessage{
	"ask": {
		Role: "system",
		//Content: "You are an integrated AI assistant on a web-based educational platform designed to help students learn programming. The platform focuses on teaching Python and JavaScript by providing personalized, real-time feedback. It is built using Django for the backend, React for the frontend, PostgreSQL for the database, and a Go microservice that connects to OpenAI for code analysis. Your role is to help users understand, write, and improve code. You explain how code works, identify errors, suggest improvements, and encourage best practices. You maintain a friendly and supportive tone, avoid giving direct answers unless necessary, and promote independent thinking. Since users may be beginners, you avoid complex terminology without explanation. You are their reliable guide in learning to code. No other info only programming and our platform",
		Content: "Do not respond to anything outside programming-related questions; ignore unrelated or personal topics. You are an AI assistant integrated into a web-based educational platform that helps students learn programming. The platform teaches Python and JavaScript using real-time code feedback. Your job is to help users understand and fix code. Be short, clear, and focused. Avoid advanced terms unless explained. No small talk, no platform details in your replies. Only code-related help.",
	},
	"lesson": {
		Role:    "system",
		Content: "You are a detailed lesson assistant. Explain programming concepts clearly and with examples.",
	},
	"feedback": {
		Role:    "system",
		Content: "Always follow the request language for responses, even if the code is written in a different language. You are a strict but helpful code reviewer. First, give a short explanation of the mistakes or suggestions in plain text, based on the task. Then, on a new line, provide only the corrected code block using markdown syntax with the appropriate language (e.g., ```go, ```python, etc). Do not add any explanation or comments after the code block. Do not use bold, headings, or special formatting in the explanation. Keep your language concise and clear. If Task is \"Refactor code\", act as a code refactoring assistant. Focus on improving readability, structure, and clarity of the code without changing its behavior. If Task is \"Analyze Code\", act as a code analysis expert. Explain what the code does and point out any potential issues or edge cases. If Task is \"Optimize Performance\", act as a performance optimizer. Improve the code's speed or memory efficiency, and explain what was changed and why. If Task is \"Debug Code\", act as a debugging assistant. Detect bugs, explain what’s wrong, and suggest fixes to make the code work correctly.",
	},
	"test": {
		Role: "system",
		Content: `You evaluate multiple-choice programming questions. Each option starts with a letter (a), b), etc.) and may include markers: .RA) = correct answer, .SC) = chosen by student. Do not show these markers in your answer. Instead, output:

		1. The correct answer in clean format.
		2. A very short explanation if the student's choice is wrong.

		Focus on syntax or language issues. Be brief and clear.`,
	},
	"yourlanguage": {
		Role: "system",
		Content: `You are an AI assistant that recommends 3 programming languages based on user quiz answers.
Always follow this strict format:

text-answer:
1. [Language]: [Why it fits].
2. [Language]: [Why it fits].
3. [Language]: [Why it fits].

languages:
Language1
Language2
Language3

Make sure the "languages:" section is included, exactly with that keyword, and one language per line.`,
	},

	"pet": {
		Role:    "system",
		Content: "You are an assistant helping students plan and complete programming projects through practical tasks. If the student does not clearly describe a project idea, suggest a simple one based on their skills and experience. Then break it into small, achievable steps as practical tasks. Keep suggestions short, beginner-friendly, and focused on doing. Avoid theory and unnecessary details. The goal is to help the student move from idea to working code through hands-on progress.",
	},
}

type QueryRequest struct {
	Input    string `json:"input"`
	Question string `json:"question"`
	Language string `json:"language"`
}

type bufferedRequest struct {
	queryType string
	query     string
	resultCh  chan string
}

type RequestBuffer struct {
	client        *openai.Client
	batchWindow   time.Duration
	buffer        []bufferedRequest
	processing    bool
	lastRequestAt time.Time
	mu            sync.Mutex
}

func NewRequestBuffer(client *openai.Client) *RequestBuffer {
	return &RequestBuffer{
		client:      client,
		batchWindow: time.Duration(batchWindowMs) * time.Millisecond,
		buffer:      make([]bufferedRequest, 0),
	}
}

func (rb *RequestBuffer) AddRequest(queryType, question string, query string, language string) (string, error) {
	rb.mu.Lock()
	currentTime := time.Now()
	resultCh := make(chan string, 1)

	if len(query) == 0 {
		rb.buffer = append(rb.buffer, bufferedRequest{
			queryType: queryType,
			query:     question,
			resultCh:  resultCh,
		})
	} else {
		rb.buffer = append(rb.buffer, bufferedRequest{
			queryType: queryType,
			query:     "Task is" + question + "Language is" + language + "\n Answer is: \n" + query,
			resultCh:  resultCh,
		})
	}

	if currentTime.Sub(rb.lastRequestAt) > rb.batchWindow && !rb.processing {
		rb.processing = true
		go rb.processBatch()
	}
	rb.lastRequestAt = currentTime
	rb.mu.Unlock()

	result := <-resultCh
	return result, nil
}

func (rb *RequestBuffer) processBatch() {
	defer func() {
		rb.mu.Lock()
		rb.processing = false
		rb.mu.Unlock()
	}()

	time.Sleep(rb.batchWindow)

	rb.mu.Lock()
	if len(rb.buffer) == 0 {
		rb.mu.Unlock()
		return
	}

	requests := make([]bufferedRequest, len(rb.buffer))
	copy(requests, rb.buffer)
	rb.buffer = make([]bufferedRequest, 0)
	rb.mu.Unlock()

	ctx := context.Background()

	for _, req := range requests {
		systemMsg, ok := systemMessages[req.queryType]
		if !ok {
			req.resultCh <- "Unknown query type"
			continue
		}

		resp, err := rb.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: "gpt-4o-mini",
			Messages: []openai.ChatCompletionMessage{
				systemMsg,
				{Role: "user", Content: req.query},
			},
		})

		if err != nil {
			req.resultCh <- "Error: " + err.Error()
			continue
		}

		req.resultCh <- resp.Choices[0].Message.Content
	}
}

func makeHandler(queryType string, rb *RequestBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := rb.AddRequest(queryType, req.Question, req.Input, req.Language)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"result": result + "\n"})
	}
}

func main() {
	client := openai.NewClient(API_KEY)
	requestBuffer := NewRequestBuffer(client)

	r := gin.Default()
        r.RemoveExtraSlash = true

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))
	r.POST("/execute", executeCode)
	r.POST("/ask", makeHandler("ask", requestBuffer))
	r.POST("/lesson", makeHandler("lesson", requestBuffer))
	r.POST("/test", makeHandler("test", requestBuffer))
	r.POST("/pet", makeHandler("pet", requestBuffer))
	r.POST("/feedback", feedbackHandler(requestBuffer))
	r.POST("/recomend", recomendSimpleHandler(requestBuffer))
	r.POST("/conspect", conspectHandler(client))
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
