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

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

// CreateAgent 创建并初始化一个带有工具调用能力的React Agent
func CreateAgent(ctx context.Context) (*react.Agent, error) {
	// 初始化模型
	var llm einomodel.ToolCallingChatModel
	var err error

	// 根据配置选择使用哪个模型
	if viper.GetString("ai.model.type") == "openai" {
		llm, err = CreateOpenAIChatModel(ctx)
	} else {
		llm, err = CreateOllamaChatModel(ctx)
	}

	if err != nil {
		return nil, err
	}

	// 加载工具
	tools := loadTools(ctx)

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
func loadTools(ctx context.Context) []tool.BaseTool {
	var tools []tool.BaseTool

	// 可以添加各种工具

	log.Printf("加载了 %d 个工具", len(tools))
	return tools
}

// NewCustomTool 创建一个自定义工具示例
func NewCustomTool() tool.BaseTool {
	return &customTool{}
}

type customTool struct{}

func (t *customTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "weave_tool",
		Desc: "这是一个自定义工具示例",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"param1": {
				Type: schema.String,
				Desc: "参数1",
			},
			"param2": {
				Type: schema.Integer,
				Desc: "参数2",
			},
		}),
	}, nil
}

func (t *customTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 工具调用的实现逻辑
	return `{"status":"ok","result":"自定义工具调用成功"}`, nil
}
