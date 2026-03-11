package main

import (
	"os"

	"ai-proxy/config"
	"ai-proxy/downstream"
	"ai-proxy/downstream/protocols"
	"ai-proxy/logging"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	logging.Init()

	var storage *logging.Storage
	if cfg.SSELogDir != "" {
		storage = logging.NewStorage(cfg.SSELogDir)
	}

	r := gin.Default()
	r.Use(logging.CaptureMiddleware(storage))

	r.GET("/health", downstream.HealthCheck)

	r.GET("/v1/models", downstream.ListModels(cfg))

	r.POST("/v1/chat/completions", downstream.StreamHandler(cfg, &protocols.OpenAIAdapter{}))

	r.POST("/v1/messages", downstream.StreamHandler(cfg, &protocols.AnthropicAdapter{}))

	r.POST("/v1/openai-to-anthropic/messages", downstream.StreamHandler(cfg, &protocols.BridgeAdapter{}))

	addr := ":" + cfg.Port
	logging.InfoMsg("ai-proxy server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		logging.ErrorMsg("Failed to start server: %v", err)
		os.Exit(1)
	}
}
