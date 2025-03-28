package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/eaddingtonwhite/momento-mcp/pkg/langchain_utils"
	"github.com/eaddingtonwhite/momento-mcp/pkg/transport"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/http"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/momentohq/client-sdk-go/auth"
	"github.com/momentohq/client-sdk-go/config"
	"github.com/momentohq/client-sdk-go/momento"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const startingPrompt = "Can you store a hello world value?"

func main() {

	var (
		mcpClient *mcpgolang.Client
		err       error
	)

	// Init MCP Client
	transportType := os.Getenv("TRANSPORT")
	switch {
	case transportType == "MOMENTO":
		mcpClient, err = initMomentoMcpClient()
	case transportType == "HTTP":
		mcpClient, err = initHttpClient()
	default:
		mcpClient, err = initStdIoMcpClientAndSever()
	}
	if err != nil {
		log.Fatal(err)
	}

	// Set up model to use
	llm, err := openai.New(openai.WithModel("gpt-3.5-turbo-0125"))
	if err != nil {
		log.Fatal(err)
	}

	// Discover available tools to use
	availableTools, err := langchain_utils.ListAvailableToolsLangChain(mcpClient)
	if err != nil {
		log.Fatal(err)
	}

	// Sending the initial message to the model, with a list of available tools.
	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, startingPrompt),
	}
	fmt.Println(startingPrompt)

	// Execute tool calls requested by the model
	messageHistory, err = langchain_utils.ExecuteToolCalls(
		context.Background(),
		mcpClient,
		llm,
		availableTools,
		messageHistory,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Send the query to the model again, this time with a history containing its
	// request to invoke a tool and our response to the tool call.
	resp, err := llm.GenerateContent(context.Background(), messageHistory, llms.WithTools(availableTools))
	if err != nil {
		log.Fatal(err)
	}

	// Print out final result returned
	fmt.Println(resp.Choices[0].Content)
}

func initHttpClient() (*mcpgolang.Client, error) {
	c := mcpgolang.NewClient(
		http.NewHTTPClientTransport("/mcp").WithBaseURL("http://localhost:8080"),
	)
	_, err := c.Initialize(context.Background())
	return c, err
}

func initMomentoMcpClient() (*mcpgolang.Client, error) {

	// Load ENV variables for initializing Momento
	cn := os.Getenv("CACHE_NAME")
	if cn == "" {
		return nil, errors.New("CACHE_NAME must be set")
	}
	credProvider, err := auth.FromEnvironmentVariable("MOMENTO_API_KEY")
	if err != nil {
		return nil, err
	}

	// Set up topic client for transport
	t, err := momento.NewTopicClient(config.TopicsDefault(), credProvider)
	if err != nil {
		return nil, err
	}

	// Create and initialize mcp client
	client := mcpgolang.NewClient(
		transport.NewMomentoClientTransport(t, cn),
	)

	// Make sure the MCP client is initialized
	if _, err := client.Initialize(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}

func initStdIoMcpClientAndSever() (*mcpgolang.Client, error) {
	cmd := exec.Command("go", "run", "../../pkg/server/main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	
	// Create and initialize client
	client := mcpgolang.NewClient(
		stdio.NewStdioServerTransportWithIO(stdout, stdin),
	)

	if _, err := client.Initialize(context.Background()); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	return client, nil
}
