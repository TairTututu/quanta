package main

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type CodeRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type CodeResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

func executeCode(c *gin.Context) {
	var req CodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tmpFile, err := ioutil.TempFile("", "code-*."+req.Language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Temp file error"})
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(req.Code)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write code"})
		return
	}
	tmpFile.Close()

	var cmd *exec.Cmd
	switch req.Language {
	case "go":
		cmd = exec.Command("go", "run", tmpFile.Name())

	case "python":
		cmd = exec.Command("python3", tmpFile.Name())

	case "javascript":
		cmd = exec.Command("node", tmpFile.Name())

	case "cpp":
		outFile := tmpFile.Name() + ".out"
		cmd = exec.Command("sh", "-c", "g++ "+tmpFile.Name()+" -o "+outFile+" && "+outFile)

	case "java":
		javaFile := tmpFile.Name()
		_ = os.Rename(javaFile, "/tmp/Main.java")
		cmd = exec.Command("sh", "-c", "javac /tmp/Main.java && java -cp /tmp Main")

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported language"})
		return
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Execution timed out"})
		return
	case err := <-done:
		exitCode := 0
		if err != nil {
			exitCode = 1
		}
		c.JSON(http.StatusOK, CodeResponse{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		})
	}
}
