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
	"strings"

	"weave/pkg"
	"weave/services/aichat/internal/tool"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// 模型支持工具调用的配置映射
var (
	supportedToolCallModels   map[string]bool
	unsupportedToolCallModels map[string]bool
)

// 初始化模型支持配置
func initModelSupportConfig() {
	// 加载工具调用模型列表
	supportedModelList := viper.GetString("SUPPORTED_TOOL_CALL_MODELS")
	unsupportedModelList := viper.GetString("UNSUPPORTED_TOOL_CALL_MODELS")

	// 初始化映射
	supportedToolCallModels = make(map[string]bool)
	unsupportedToolCallModels = make(map[string]bool)

	// 解析支持的模型列表
	if supportedModelList != "" {
		for _, model := range SplitString(supportedModelList, ",") {
			if model = TrimSpace(model); model != "" {
				supportedToolCallModels[model] = true
			}
		}
	}

	// 解析不支持的模型列表
	if unsupportedModelList != "" {
		for _, model := range SplitString(unsupportedModelList, ",") {
			if model = TrimSpace(model); model != "" {
				unsupportedToolCallModels[model] = true
			}
		}
	}
}

// SplitString 分割字符串
func SplitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, sep)
}

// TrimSpace 去除字符串两端空白
func TrimSpace(s string) string {
	return strings.TrimSpace(s)
}

// CreateAgent 创建并初始化一个React Agent
func CreateAgent(ctx context.Context) (*react.Agent, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// 获取日志实例
	logger := pkg.GetLogger()
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn("未找到 .env 文件或读取失败，将使用环境变量或默认值", zap.Error(err))
	}

	// 初始化模型支持配置
	initModelSupportConfig()

	// 初始化模型
	var llm einomodel.ToolCallingChatModel
	var err error
	var modelName string

	// 根据配置类型选择模型
	modelType := viper.GetString("AI_MODEL_TYPE")
	if modelType == "openai" {
		llm, err = CreateOpenAIChatModel(ctx)
		modelName = viper.GetString("OPENAI_MODEL_NAME")
	} else if modelType == "modelscope" {
		llm, err = CreateModelScopeChatModel(ctx)
		modelName = viper.GetString("MODELSCOPE_MODEL_NAME")
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
		logger.Info("当前模型支持工具调用", zap.String("model_name", modelName), zap.Int("tool_count", len(tools)))
	} else {
		tools = []einotool.BaseTool{}
		logger.Info("当前模型不支持工具调用，将以普通对话模式运行", zap.String("model_name", modelName))
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
	if unsupportedToolCallModels != nil && unsupportedToolCallModels[modelName] {
		return false
	}
	// 再检查支持的列表
	if supportedToolCallModels != nil && supportedToolCallModels[modelName] {
		return true
	}
	// 默认不支持
	return false
}
