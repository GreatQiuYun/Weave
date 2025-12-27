/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"context"
	"log"

	"weave/services/aichat/internal/tool"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/spf13/viper"
)

// 变量 模型是否支持工具调用
var (
	// 不支持工具调用的模型
	unsupportedToolCallModels = map[string]bool{
		// DeepSeek 模型
		"deepseek-r1":   true,
		"deepseek-v3":   true,
		"deepseek-math": true,

		// Ollama 基础模型
		"llama2":     true,
		"llama2:7b":  true,
		"llama2:13b": true,
		"llama2:70b": true,

		// 非对话优化模型
		"codellama":     true,
		"codellama:7b":  true,
		"codellama:13b": true,
		"codellama:34b": true,
		"codellama:70b": true,

		"stablelm2":      true,
		"stablelm2:1.6b": true,
		"stablelm2:12b":  true,

		"qwen2:0.5b": true,
		"qwen2:1.5b": true,
		"qwen2:7b":   true,

		"gemma2:2b": true,
		"gemma2:9b": true,

		"phi3":        true,
		"phi3:mini":   true,
		"phi3:medium": true,

		// 其他基础模型
		"mixtral:8x7b":  true,
		"mixtral:8x22b": true,

		// GPT 非对话模型
		"gpt-3.5-turbo-instruct": true,
		"text-davinci-003":       true,
		"dall-e":                 true,

		// Claude 非对话模型
		"claude-instant-1": true,

		// 嵌入模型
		"text-embedding-ada-002": true,
		"text-embedding-3-small": true,
		"text-embedding-3-large": true,
	}

	// 支持工具调用的模型
	supportedToolCallModels = map[string]bool{
		// OpenAI GPT 系列
		"gpt-3.5-turbo":          true,
		"gpt-3.5-turbo-0125":     true,
		"gpt-3.5-turbo-1106":     true,
		"gpt-4":                  true,
		"gpt-4-0613":             true,
		"gpt-4-0125-preview":     true,
		"gpt-4-turbo":            true,
		"gpt-4-turbo-2024-04-09": true,
		"gpt-4o":                 true,
		"gpt-4o-mini":            true,

		// Anthropic Claude 系列
		"claude-2":          true,
		"claude-2.1":        true,
		"claude-3-opus":     true,
		"claude-3-sonnet":   true,
		"claude-3-haiku":    true,
		"claude-3-5-sonnet": true,

		// Ollama 支持工具调用的模型
		"llama3.1":               true,
		"llama3.1:8b":            true,
		"llama3.1:8b-instruct":   true,
		"llama3.1:70b":           true,
		"llama3.1:70b-instruct":  true,
		"llama3.1:405b":          true,
		"llama3.1:405b-instruct": true,

		"llama3.2":             true,
		"llama3.2:1b":          true,
		"llama3.2:3b":          true,
		"llama3.2:1b-instruct": true,
		"llama3.2:3b-instruct": true,

		"mistral:7b-instruct-v0.2":        true,
		"mistral:7b-instruct-v0.3":        true,
		"mistral-small:22b-instruct-v0.1": true,
		"mistral-small:22b-instruct-v0.2": true,
		"mistral-small3.1":                true,
		"mistral-small3.1:22b":            true,
		"mistral-small3.1:22b-instruct":   true,
		"mistral-nemo":                    true,
		"mistral-nemo:12b-instruct":       true,

		// DeepSeek 支持工具调用的版本
		"deepseek-chat":     true,
		"deepseek-coder":    true,
		"deepseek-v2-chat":  true,
		"deepseek-v2-coder": true,

		// Google Gemini 系列
		"gemini-pro":       true,
		"gemini-pro-1.5":   true,
		"gemini-1.5-flash": true,
		"gemini-1.5-pro":   true,

		// 其他支持工具调用的模型
		"qwen2:7b-instruct":          true,
		"qwen2:14b-instruct":         true,
		"qwen2:32b-instruct":         true,
		"qwen2:72b-instruct":         true,
		"qwen2.5:7b-instruct":        true,
		"qwen2.5:14b-instruct":       true,
		"qwen2.5:32b-instruct":       true,
		"qwen2.5:72b-instruct":       true,
		"qwen2.5-coder:7b-instruct":  true,
		"qwen2.5-coder:14b-instruct": true,
		"qwen2.5-coder:32b-instruct": true,

		"codestral:22b-instruct":      true,
		"codestral:mamba-7b-instruct": true,

		"gemma2:2b-instruct":  true,
		"gemma2:9b-instruct":  true,
		"gemma2:27b-instruct": true,

		"phi3:mini-instruct":   true,
		"phi3:medium-instruct": true,
		"phi3:vision-instruct": true,

		// 阿里云系列
		"qwen-plus":  true,
		"qwen-turbo": true,
		"qwen-max":   true,

		// 智谱AI系列
		"glm-4":  true,
		"glm-4v": true,

		// 月之暗面系列
		"moonshot-v1-8k":   true,
		"moonshot-v1-32k":  true,
		"moonshot-v1-128k": true,
	}
)

// CreateAgent 创建并初始化一个React Agent
func CreateAgent(ctx context.Context) (*react.Agent, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("未找到 .env 文件或读取失败: %v，将使用环境变量或默认值", err)
	}

	// 初始化模型
	var llm einomodel.ToolCallingChatModel
	var err error
	var modelName string

	// 根据配置类型选择模型
	if viper.GetString("ai.model.type") == "openai" {
		llm, err = CreateOpenAIChatModel(ctx)
		modelName = viper.GetString("OPENAI_MODEL_NAME")
	} else {
		llm, err = CreateOllamaChatModel(ctx)
		modelName = viper.GetString("OLLAMA_MODEL_NAME")
	}

	if err != nil {
		return nil, err
	}

	// 检查模型是否支持工具调用
	var tools []einotool.BaseTool
	if isModelSupportToolCall(modelName) {
		tools = loadTools(ctx)
		log.Printf("当前模型 %s 支持工具调用，已加载 %d 个工具", modelName, len(tools))
	} else {
		tools = []einotool.BaseTool{}
		log.Printf("当前模型 %s 不支持工具调用，将以普通对话模式运行", modelName)
	}

	// 创建React Agent
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: llm,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: tools,
		},
	})

	if err != nil {
		return nil, err
	}

	return agent, nil
}

// loadTools 加载所有可用的工具
func loadTools(ctx context.Context) []einotool.BaseTool {
	var tools []einotool.BaseTool

	// 添加工具
	tools = append(tools, tool.NewCustomTool())

	return tools
}

// isModelSupportToolCall 检查模型是否支持工具调用
func isModelSupportToolCall(modelName string) bool {
	// 检索列表
	if unsupportedToolCallModels[modelName] {
		return false
	}
	if supportedToolCallModels[modelName] {
		return true
	}

	return false
}
