package langchain_utils

import (
	"context"
	"encoding/json"
	mcpgolang "github.com/metoro-io/mcp-golang"
	"github.com/tmc/langchaingo/llms"
	"log"
)

func ListAvailableToolsLangChain(mcpClient *mcpgolang.Client) ([]llms.Tool, error) {
	// List available tools from MCP server
	rsp, err := mcpClient.ListTools(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	// Convert MCP tool response to Langchain tool response type's
	var tools []llms.Tool
	for _, tool := range rsp.Tools {
		tools = append(tools, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        tool.Name,
				Description: *tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return tools, nil
}

func ExecuteToolCalls(
	ctx context.Context,
	mcpClient *mcpgolang.Client,
	llm llms.Model,
	tools []llms.Tool,
	messageHistory []llms.MessageContent,
) ([]llms.MessageContent, error) {

	resp, err := llm.GenerateContent(
		ctx, messageHistory, llms.WithTools(tools),
	)
	if err != nil {
		log.Fatal(err)
	}
	messageHistory = updateMessageHistoryWithToolCalls(messageHistory, resp)

	// Determine all the tool calls to make
	for _, toolCall := range resp.Choices[0].ToolCalls {

		// Unmarshal from JSON args to map[string]any for call to MCP
		var v map[string]any
		err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &v)
		if err != nil {
			return nil, err
		}

		// Execute function Call to MCP
		rsp, err := mcpClient.CallTool(ctx, toolCall.FunctionCall.Name, v)
		if err != nil {
			return nil, err
		}

		// Parse out response from MCP and translate to Langchain tool response and append to chat history
		messageHistory = append(messageHistory, llms.MessageContent{
			Role: llms.ChatMessageTypeTool,
			Parts: []llms.ContentPart{
				llms.ToolCallResponse{
					ToolCallID: toolCall.ID,
					Name:       toolCall.FunctionCall.Name,
					// TODO make this more flexible on how content returned going between MCP and Langchain resp type
					Content: rsp.Content[0].TextContent.Text,
				},
			},
		})
	}

	return messageHistory, nil
}

// updateMessageHistoryWithToolCalls updates the message history with the assistant's
// response and requested tool calls.
func updateMessageHistoryWithToolCalls(messageHistory []llms.MessageContent, resp *llms.ContentResponse) []llms.MessageContent {
	respchoice := resp.Choices[0]

	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
	for _, tc := range respchoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	return append(messageHistory, assistantResponse)
}
