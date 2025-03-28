# momento-mcp

A Model Context Protocol server for interacting with Momento services. 

### Currently Supported API's

| Command | Description                            | Input                                              |
|---------|----------------------------------------|----------------------------------------------------|
| `set`   | Set a key-value pair with optional TTL | `key` (string), `value` (string), `ttl` (int, optional) |
| `get`   | Get value by key                       | `key` (string)                                     |
| `delete`| Delete an item by key                  | `key` (string)                                     |

## Running Server & Examples

### With Claude Desktop

1.  Build Momento MCP Server
`go build pkg/server/main.go`
2. Get a Momento API Key from [Momento Console](https://console.gomomento.com) 
3. Configure Claude Desktop App's `claude_desktop_config.json`
```json
{
    "mcpServers": {
        "momento-mcp-server": {
            "command": "/Path/to/project/momento-mcp/main",
            "args": [],
            "env": {
                "MOMENTO_API_KEY": "REPLACE_ME",
                "CACHE_NAME": "default"
            }
        }
    }
}
```

### Running Examples

_Note: Example requires Momento and Open AI API keys set_

#### HTTP Transport
1. Start Server
```
TRANSPORT=HTTP go run pkg/server/main.go
```
2. Run Client
```
cd examples/langchain
TRANSPORT=HTTP go run main.go
```

#### Momento Transport
1. Start Server
```
TRANSPORT=MOMENTO go run pkg/server/main.go
```
2. Run Client
```
cd examples/langchain
TRANSPORT=MOMENTO go run main.go
```

