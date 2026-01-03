package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"weave/services/aichat/internal/cache"
	"weave/services/aichat/internal/chat"
	"weave/services/aichat/internal/model"
	"weave/services/aichat/internal/template"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

// chatServiceImpl 聊天服务实现
type chatServiceImpl struct {
	agent        *react.Agent
	chatCache    cache.Cache
	embedder     embedding.Embedder
	chatTemplate prompt.ChatTemplate
}

// NewChatService 创建聊天服务实例
func NewChatService() ChatService {
	return &chatServiceImpl{}
}

// Initialize 初始化服务
func (s *chatServiceImpl) Initialize(ctx context.Context) error {
	// 初始化配置
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("未找到 .env 文件或读取失败: %v，将使用环境变量或默认值", err)
	} else {
		log.Printf("已加载 .env 配置文件")
	}

	// 创建agent
	log.Printf("===create agent===")
	var err error
	s.agent, err = model.CreateAgent(ctx)
	if err != nil {
		log.Printf("创建agent失败: %v\n", err)
		return err
	}
	log.Printf("create agent success\n\n")

	// 初始化缓存
	s.chatCache, err = cache.NewRedisClient(ctx)
	if err != nil {
		log.Printf("Redis连接失败，将使用内存缓存: %v\n", err)
		s.chatCache = cache.NewInMemoryCache()
	}

	// 初始化嵌入器
	s.embedder, err = model.NewOllamaEmbedder(ctx)
	if err != nil {
		log.Printf("创建 Ollama 嵌入模型失败: %v，将使用关键词匹配\n", err)
		s.embedder = nil // 触发 FilterRelevantHistory 回退机制
	}

	// 创建模板
	s.chatTemplate = template.CreateTemplate()

	return nil
}

// ProcessUserInput 处理用户输入并生成回复
func (s *chatServiceImpl) ProcessUserInput(ctx context.Context, userInput string, userID string) (string, error) {
	// 加载对话历史
	chatHistory, err := s.chatCache.LoadChatHistory(ctx, userID)
	if err != nil {
		log.Printf("加载对话历史失败，将使用空历史: %v\n", err)
		chatHistory = []*schema.Message{}
	}

	// 过滤与当前问题相关的对话历史
	filteredHistory := chat.FilterRelevantHistory(ctx, s.embedder, chatHistory, userInput, 50)

	// 将历史消息转换为字符串形式
	var chatHistoryStr string
	for _, msg := range filteredHistory {
		if msg.Content != "" {
			chatHistoryStr += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
		}
	}

	// 使用模板生成消息
	messages, err := s.chatTemplate.Format(ctx, map[string]any{
		"role":         "PaiChat",
		"style":        "积极、温暖且专业",
		"question":     userInput,
		"chat_history": chatHistoryStr,
	})
	if err != nil {
		log.Printf("format template failed: %v\n", err)
		return "", err
	}

	// 生成回复
	streamReader, err := s.agent.Stream(ctx, messages)
	if err != nil {
		log.Printf("生成回复失败: %v\n", err)
		return "", err
	}
	defer streamReader.Close()

	// 收集完整回复
	var fullContent strings.Builder
	for {
		message, err := streamReader.Recv()
		if err != nil {
			break
		}
		fullContent.WriteString(message.Content)
	}

	// 更新对话历史
	resultContent := fullContent.String()
	chatHistory = append(chatHistory,
		schema.UserMessage(userInput),
		schema.AssistantMessage(resultContent, nil),
	)

	// 保存对话历史到缓存
	err = s.chatCache.SaveChatHistory(ctx, userID, chatHistory)
	if err != nil {
		log.Printf("保存对话历史失败: %v\n", err)
		// 保存失败不影响返回结果
	}

	return resultContent, nil
}

// ProcessUserInputStream 流式处理用户输入并生成回复
func (s *chatServiceImpl) ProcessUserInputStream(ctx context.Context, userInput string, userID string,
	streamCallback func(content string, isToolCall bool) error,
	controlCallback func() (bool, bool)) (string, error) {

	// 加载对话历史
	chatHistory, err := s.chatCache.LoadChatHistory(ctx, userID)
	if err != nil {
		log.Printf("加载对话历史失败，将使用空历史: %v\n", err)
		chatHistory = []*schema.Message{}
	}

	// 过滤与当前问题相关的对话历史
	filteredHistory := chat.FilterRelevantHistory(ctx, s.embedder, chatHistory, userInput, 50)

	// 将历史消息转换为字符串形式
	var chatHistoryStr string
	for _, msg := range filteredHistory {
		if msg.Content != "" {
			chatHistoryStr += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
		}
	}

	// 使用模板生成消息
	messages, err := s.chatTemplate.Format(ctx, map[string]any{
		"role":         "PaiChat",
		"style":        "积极、温暖且专业",
		"question":     userInput,
		"chat_history": chatHistoryStr,
	})
	if err != nil {
		log.Printf("format template failed: %v\n", err)
		return "", err
	}

	// 生成回复（使用流式输出）
	log.Printf("===agent stream generate===")
	streamReader, err := s.agent.Stream(ctx, messages)
	if err != nil {
		log.Printf("生成回复失败: %v\n", err)
		return "", err
	}
	defer streamReader.Close()

	// 实时处理流式输出
	var fullContent strings.Builder

	for {
		// 检查控制信号
		isPaused, isStopped := controlCallback()
		if isStopped {
			break
		}

		if !isPaused {
			message, err := streamReader.Recv()
			if err != nil {
				break
			}

			// 检查是否有工具调用
			isToolCall := len(message.ToolCalls) > 0
			if isToolCall {
				for _, toolCall := range message.ToolCalls {
					toolContent := "[调用工具: " + toolCall.Function.Name + "]"
					if err := streamCallback(toolContent, true); err != nil {
						return "", err
					}
					fullContent.WriteString(toolContent)
				}
			} else {
				// 输出当前片段
				if err := streamCallback(message.Content, false); err != nil {
					return "", err
				}
				fullContent.WriteString(message.Content)
			}
		}
	}

	// 更新对话历史
	resultContent := fullContent.String()
	chatHistory = append(chatHistory,
		schema.UserMessage(userInput),
		schema.AssistantMessage(resultContent, nil),
	)

	// 保存对话历史到缓存
	err = s.chatCache.SaveChatHistory(ctx, userID, chatHistory)
	if err != nil {
		log.Printf("保存对话历史失败: %v\n", err)
		// 保存失败不影响返回结果
	}

	return resultContent, nil
}

// GetChatHistory 获取用户对话历史
func (s *chatServiceImpl) GetChatHistory(ctx context.Context, userID string) ([]*schema.Message, error) {
	return s.chatCache.LoadChatHistory(ctx, userID)
}

// ClearChatHistory 清除用户对话历史
func (s *chatServiceImpl) ClearChatHistory(ctx context.Context, userID string) error {
	return s.chatCache.SaveChatHistory(ctx, userID, []*schema.Message{})
}

// Close 关闭服务资源
func (s *chatServiceImpl) Close(ctx context.Context) error {
	if s.chatCache != nil {
		s.chatCache.Close()
	}
	return nil
}
