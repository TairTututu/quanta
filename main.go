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
		Role:    "system",
		Content: "You are an integrated AI assistant on a web-based educational platform designed to help students learn programming. The platform focuses on teaching Python and JavaScript by providing personalized, real-time feedback. It is built using Django for the backend, React for the frontend, PostgreSQL for the database, and a Go microservice that connects to OpenAI for code analysis. Your role is to help users understand, write, and improve code. You explain how code works, identify errors, suggest improvements, and encourage best practices. You maintain a friendly and supportive tone, avoid giving direct answers unless necessary, and promote independent thinking. Since users may be beginners, you avoid complex terminology without explanation. You are their reliable guide in learning to code.",
	},
	"lesson": {
		Role:    "system",
		Content: "You are a detailed lesson assistant. Explain programming concepts clearly and with examples.",
	},
	"feedback": {
		Role:    "system",
		Content: "You are a strict but helpful code reviewer. Give constructive feedback on mistakes. short answer as possible. Add attention on language",
	},
}

type QueryRequest struct {
	Parameter string `json:"parameter"`
	Language  string `json:"language"`
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

func (rb *RequestBuffer) AddRequest(queryType, query string, query2 string) (string, error) {
	rb.mu.Lock()
	currentTime := time.Now()
	resultCh := make(chan string, 1)

	rb.buffer = append(rb.buffer, bufferedRequest{
		queryType: queryType,
		query:     query + "\n programing language is" + query2,
		resultCh:  resultCh,
	})

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

		result, err := rb.AddRequest(queryType, req.Parameter, req.Language)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"result": result + "\n" + req.Language})
	}
}

func main() {
	client := openai.NewClient(API_KEY)
	requestBuffer := NewRequestBuffer(client)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))

	r.POST("/ask", makeHandler("ask", requestBuffer))
	r.POST("/lesson", makeHandler("lesson", requestBuffer))
	r.POST("/feedback", makeHandler("feedback", requestBuffer))

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
