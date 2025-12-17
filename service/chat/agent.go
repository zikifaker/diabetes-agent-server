package chat

import (
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/request"
	"diabetes-agent-backend/utils"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	mcpadapter "github.com/i2y/langchaingo-mcp-adapter"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
)

const (
	BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

	methodToolCompleted = "tool_completed"
)

var (
	// 配置 300s 超时时间处理 LLM 流式输出
	agentHTTPClient *http.Client = utils.NewHTTPClient(
		utils.WithTimeout(300 * time.Second),
	)

	mcpHTTPClient *http.Client = utils.DefaultHTTPClient()
)

var (
	//go:embed prompts/conversational_format_instructions.txt
	conversationalFormatInstructions string

	//go:embed prompts/conversational_prefix.txt
	conversationalPrefix string

	//go:embed prompts/conversational_suffix.txt
	conversationalSuffix string
)

type Agent struct {
	Executor    *agents.Executor
	MCPClient   *client.Client
	ChatHistory *MySQLChatMessageHistory
	SSEHandler  *GinSSEHandler
}

func NewAgent(c *gin.Context, req request.ChatRequest) (*Agent, error) {
	llm, err := openai.New(
		openai.WithModel(req.AgentConfig.Model),
		openai.WithToken(config.Cfg.Model.APIKey),
		openai.WithBaseURL(BaseURL),
		openai.WithHTTPClient(agentHTTPClient),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm client: %v", err)
	}

	mcpClient, err := createMCPClient(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create mcp client: %v", err)
	}

	ctx := context.Background()
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to init connection to the mcp server: %v", err)
	}

	mcpTools, err := getMCPTools(mcpClient, req.AgentConfig.Tools)
	if err != nil {
		slog.Error("failed to get mcp tools", "err", err)
	}

	sseHandler := NewGinSSEHandler(c, req.SessionID)
	registerMCPClientNotifications(ctx, mcpClient, sseHandler)

	a := agents.NewConversationalAgent(llm, mcpTools,
		agents.WithCallbacksHandler(sseHandler),
		agents.WithPromptPrefix(conversationalPrefix),
		agents.WithPromptFormatInstructions(conversationalFormatInstructions),
		agents.WithPromptSuffix(conversationalSuffix),
	)

	chatHistory := NewMySQLChatMessageHistory(req.SessionID)
	memory := memory.NewConversationBuffer(
		memory.WithChatHistory(chatHistory),
	)

	executor := agents.NewExecutor(
		a,
		agents.WithMemory(memory),
		agents.WithMaxIterations(req.AgentConfig.MaxIterations),
	)

	return &Agent{
		Executor:    executor,
		MCPClient:   mcpClient,
		ChatHistory: chatHistory,
		SSEHandler:  sseHandler,
	}, nil
}

func (a *Agent) Call(ctx context.Context, req request.ChatRequest) (string, error) {
	result, err := chains.Run(ctx, a.Executor, req.Query)
	if err != nil {
		return "", err
	}
	return result, nil
}

// SaveAgentSteps 存储思考步骤
func (a *Agent) SaveAgentSteps(ctx context.Context) error {
	immediateSteps := a.SSEHandler.GetImmediateSteps()
	return a.ChatHistory.SetImmediateSteps(ctx, immediateSteps)
}

func (a *Agent) Close() error {
	if a.MCPClient != nil {
		return a.MCPClient.Close()
	}
	return nil
}

func createMCPClient(c *gin.Context) (*client.Client, error) {
	mcpServerPath := fmt.Sprintf("http://%s:%s/mcp", config.Cfg.MCP.Host, config.Cfg.MCP.Port)
	mcpClient, err := client.NewStreamableHttpClient(mcpServerPath,
		transport.WithHTTPBasicClient(mcpHTTPClient),
		transport.WithHTTPHeaders(map[string]string{
			"Authorization": c.GetHeader("Authorization"),
		}),
		transport.WithContinuousListening(),
	)
	if err != nil {
		return nil, err
	}
	return mcpClient, nil
}

// 返回用户选择的工具
func getMCPTools(mcpClient *client.Client, toolNames []string) ([]tools.Tool, error) {
	if len(toolNames) == 0 {
		return nil, nil
	}

	// 初始化与 MCP 服务端的连接
	mcpAdapter, err := mcpadapter.New(mcpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create mcp adapter: %v", err)
	}

	mcpTools, err := mcpAdapter.Tools()
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp tools: %v", err)
	}

	toolMap := make(map[string]bool)
	for _, name := range toolNames {
		toolMap[name] = true
	}

	var filteredTools []tools.Tool
	for _, tool := range mcpTools {
		if toolMap[tool.Name()] {
			filteredTools = append(filteredTools, tool)
		}
	}

	return filteredTools, nil
}

// 注册通知处理方法，接收 MCP 服务端推送的工具调用结果
func registerMCPClientNotifications(ctx context.Context, mcpClient *client.Client, sseHandler *GinSSEHandler) {
	mcpClient.OnNotification(func(notification mcp.JSONRPCNotification) {
		if notification.Method != methodToolCompleted {
			return
		}

		results, ok := notification.Params.AdditionalFields["result"].([]any)
		if !ok {
			slog.Error("invalid tool call result type")
			return
		}

		for _, res := range results {
			if content, ok := res.(map[string]any); ok {
				switch contentType := content["type"].(string); contentType {
				case "text":
					textContent := content["text"].(string)
					sseHandler.HandleToolEnd(ctx, textContent)
				}
			}
		}
	})
}
