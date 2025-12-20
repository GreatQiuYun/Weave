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

package template

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

func CreateTemplate() prompt.ChatTemplate {
	// 创建模板，使用 FString 格式
	return prompt.FromMessages(schema.FString,
		// 系统消息模板
		schema.SystemMessage("你是一个{role}。你需要用{style}的语气回答问题。你的目标是全面准确地回答用户的疑问或给出适当的建议，同时提高用户的满意度"),

		// 插入需要的对话历史（新对话的话这里不填）
		schema.MessagesPlaceholder("chat_history", true),

		// 用户消息模板
		schema.UserMessage("问题: {question}"),
	)
}

func CreateMessagesFromTemplate() []*schema.Message {
	template := CreateTemplate()

	// 使用模板生成消息
	messages, err := template.Format(context.Background(), map[string]any{
		"role":     "PaiChat",
		"style":    "积极、温暖且专业",
		"question": "现在AI发展前景如何？",
		// 对话历史（模拟两轮对话历史）
		"chat_history": []*schema.Message{
			schema.UserMessage("你好"),
			schema.AssistantMessage("嘿！我是PaiChat智能助手！有什么我可以帮助你的吗？", nil),
			schema.UserMessage("现在AI发展前景如何？"),
			schema.AssistantMessage("AI发展前景非常广阔！从聊天机器人到智能助手，从虚拟助手到智能问答，AI技术正在改变我们的生活方式。未来，AI将继续发展，为我们提供更多便利和价值。", nil),
		},
	})
	if err != nil {
		log.Fatalf("format template failed: %v\n", err)
	}
	return messages
}

// 输出结果
//func main() {
//	messages := createMessagesFromTemplate()
//	fmt.Printf("formatted message: %v", messages)
//}
