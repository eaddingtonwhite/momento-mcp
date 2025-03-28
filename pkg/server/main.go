package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eaddingtonwhite/momento-mcp/internal/utils"
	momentoTransport "github.com/eaddingtonwhite/momento-mcp/pkg/transport"
	"github.com/gin-gonic/gin"
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/metoro-io/mcp-golang/transport/http"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/momentohq/client-sdk-go/auth"
	"github.com/momentohq/client-sdk-go/momento"
	"github.com/momentohq/client-sdk-go/responses"
)

func main() {
	log.SetOutput(os.Stderr)
	mcpServer := mustCreateMomentoMCPServer()
	mcpServer.MustRegisterTools([]Tool{
		{
			Name:        "get",
			Description: "Get value by key from Momento",
			Handler:     mcpServer.HandleGet,
		},
		{
			Name:        "set",
			Description: "Set a key-value pair in Momento with optional TTL for expiration time",
			Handler:     mcpServer.HandleSet,
		},
		{
			Name:        "delete",
			Description: "Delete an item from Momento",
			Handler:     mcpServer.HandleDelete,
		},
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		mcpServer.ServerMustStart()
	}()

	<-ctx.Done()
	log.Println("Shutting down server gracefully...")
}

type bootStrapConfig struct {
	momentoCredProvider auth.CredentialProvider
	transportType       string
	cacheName           string
}

func mustLoadConfig() bootStrapConfig {

	transportType := os.Getenv("TRANSPORT")
	if transportType == "" {
		transportType = "stdio"
	}

	cn := os.Getenv("CACHE_NAME")
	if cn == "" {
		log.Fatal("CACHE_NAME must be set")
	}

	credProvider, err := auth.FromEnvironmentVariable("MOMENTO_API_KEY")
	if err != nil {
		log.Fatal("failed to initialize momento cred provider", err)
	}
	return bootStrapConfig{
		momentoCredProvider: credProvider,
		transportType:       transportType,
		cacheName:           cn,
	}
}

type MomentoMcpServer struct {
	transportType         string
	momentoCacheClient    momento.CacheClient
	cacheName             string
	internalBaseMcpServer *mcp_golang.Server
	ginRouter             *gin.Engine
}

func mustCreateMomentoMCPServer() MomentoMcpServer {
	startUpConfig := mustLoadConfig()
	mcc, mtc := utils.MustCreateMomentoClients(startUpConfig.momentoCredProvider)

	mcpServer := MomentoMcpServer{
		transportType:      startUpConfig.transportType,
		momentoCacheClient: mcc,
		cacheName:          startUpConfig.cacheName,
	}
	// Create MCP server transport type configurable default to stdio
	var t transport.Transport
	switch startUpConfig.transportType {
	case "HTTP":
		t = http.NewGinTransport()
		gt, _ := t.(*http.GinTransport)
		// Create a Gin router
		r := gin.Default()
		// Add the MCP endpoint
		r.POST("/mcp", gt.Handler())
		mcpServer.ginRouter = r
	case "MOMENTO":
		t = momentoTransport.NewMomentoServerTransport(mtc, startUpConfig.cacheName)
	default:
		t = stdio.NewStdioServerTransport()
	}

	// Startup base MCP server
	server := mcp_golang.NewServer(t)
	mcpServer.internalBaseMcpServer = server

	return mcpServer
}

func (m MomentoMcpServer) ServerMustStart() {
	if m.transportType == "HTTP" {
		go m.internalBaseMcpServer.Serve()
		if err := m.ginRouter.Run(":8080"); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		err := m.internalBaseMcpServer.Serve()
		if err != nil {
			panic(err)
		}
	}
}

func (m MomentoMcpServer) MustRegisterTools(tools []Tool) {
	for _, tool := range tools {
		err := m.internalBaseMcpServer.RegisterTool(
			tool.Name,
			tool.Description,
			tool.Handler,
		)
		if err != nil {
			panic(err)
		}
	}
}

func (m MomentoMcpServer) HandleSet(arguments SetArgs) (*mcp_golang.ToolResponse, error) {
	_, err := m.momentoCacheClient.Set(context.Background(), &momento.SetRequest{
		CacheName: m.cacheName,
		Key:       momento.String(arguments.Key),
		Value:     momento.String(arguments.Value),
		Ttl:       time.Duration(arguments.TTL) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(
		mcp_golang.NewTextContent(""),
	), nil
}

func (m MomentoMcpServer) HandleGet(arguments GetArgs) (*mcp_golang.ToolResponse, error) {
	rsp, err := m.momentoCacheClient.Get(context.Background(), &momento.GetRequest{
		CacheName: m.cacheName,
		Key:       momento.String(arguments.Key),
	})
	if err != nil {
		return nil, err
	}
	finalRsp := ""
	switch r := rsp.(type) {
	case *responses.GetHit:
		finalRsp = r.ValueString()

	}
	return mcp_golang.NewToolResponse(
		mcp_golang.NewTextContent(finalRsp),
	), nil
}

func (m MomentoMcpServer) HandleDelete(arguments DeleteArgs) (*mcp_golang.ToolResponse, error) {
	_, err := m.momentoCacheClient.Delete(context.Background(), &momento.DeleteRequest{
		CacheName: m.cacheName,
		Key:       momento.String(arguments.Key),
	})
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(
		mcp_golang.NewTextContent(""),
	), nil
}

type GetArgs struct {
	Key string `json:"key" jsonschema:"required,description=Key of item to get"`
}

type SetArgs struct {
	Key   string `json:"key" jsonschema:"required,description=Key of item to set"`
	Value string `json:"value" jsonschema:"required,description=value of item to set"`
	TTL   int    `json:"ttl" jsonschema:"description=TTL or expiry of item to set in seconds"`
}

type DeleteArgs struct {
	Key string `json:"key" jsonschema:"required,description=Key of item to delete"`
}

type name interface {
}

type Tool struct {
	Name        string
	Description string
	Handler     any
}
