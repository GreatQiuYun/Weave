package model

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/spf13/viper"
)

// CreateModelScopeChatModel 创建并返回一个 ModelScope 聊天模型实例
// ModelScope API 兼容 OpenAI API
func CreateModelScopeChatModel(ctx context.Context) (einomodel.ToolCallingChatModel, error) {
	key := viper.GetString("MODELSCOPE_API_KEY")
	modelName := viper.GetString("MODELSCOPE_MODEL_NAME")
	baseURL := viper.GetString("MODELSCOPE_BASE_URL")

	if key == "" || modelName == "" || baseURL == "" {
		return nil, fmt.Errorf("MODELSCOPE_API_KEY、MODELSCOPE_MODEL_NAME 或 MODELSCOPE_BASE_URL 未在 .env 文件中配置")
	}

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
		APIKey:  key,
	})
	if err != nil {
		return nil, err
	}
	return chatModel, nil
}
