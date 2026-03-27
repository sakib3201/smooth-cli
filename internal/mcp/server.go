package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/smoothcli/smooth-cli/internal/config"
)

var ErrToolDisabled = errors.New("tool disabled: set settings.mcp.dangerous_tools = true to enable")

type SupervisorReader interface {
	State(name string) (map[string]interface{}, error)
	States() map[string]map[string]interface{}
	Restart(ctx context.Context, name string) error
	SendInput(ctx context.Context, name string, input []byte) error
}

type LogReader interface {
	Lines(process string, n int) []interface{}
}

type StateReader interface {
	ActiveAttentionEvents() []interface{}
}

type Deps struct {
	Supervisor SupervisorReader
	LogStore   LogReader
	Store      StateReader
	Config     *config.SmoothConfig
}

func New(deps Deps) *server.MCPServer {
	s := server.NewMCPServer("smooth-cli", "1.0.0")

	s.AddTool(mcp.NewTool("list_processes",
		mcp.WithDescription("List all managed processes and their current status"),
	), makeListProcesses(deps))

	s.AddTool(mcp.NewTool("get_logs",
		mcp.WithDescription("Get recent log lines for a process"),
	), makeGetLogs(deps))

	s.AddTool(mcp.NewTool("get_attention_events",
		mcp.WithDescription("Get unresolved attention events"),
	), makeGetAttentionEvents(deps))

	s.AddTool(mcp.NewTool("get_config",
		mcp.WithDescription("Get current smooth.yml config"),
	), makeGetConfig(deps))

	if deps.Config != nil && deps.Config.Settings.MCP.DangerousTools {
		s.AddTool(mcp.NewTool("restart_process",
			mcp.WithDescription("Restart a managed process"),
		), makeRestartProcess(deps))
		s.AddTool(mcp.NewTool("run_command",
			mcp.WithDescription("Send input text to a process PTY"),
		), makeRunCommand(deps))
	}

	return s
}

func makeListProcesses(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		states := deps.Supervisor.States()
		result := make([]map[string]interface{}, 0, len(states))
		for _, state := range states {
			result = append(result, state)
		}
		data, _ := json.Marshal(map[string]interface{}{"processes": result})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeGetLogs(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := req.Params.Arguments["process_name"].(string)
		if name == "" {
			return nil, errors.New("process_name required")
		}
		lines := deps.LogStore.Lines(name, 100)
		data, _ := json.Marshal(map[string]interface{}{"lines": lines})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeGetAttentionEvents(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		events := deps.Store.ActiveAttentionEvents()
		data, _ := json.Marshal(map[string]interface{}{"events": events})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeGetConfig(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, _ := json.Marshal(deps.Config)
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeRestartProcess(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := req.Params.Arguments["process_name"].(string)
		if name == "" {
			return nil, errors.New("process_name required")
		}
		if err := deps.Supervisor.Restart(ctx, name); err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(`{"status":"restarting"}`), nil
	}
}

func makeRunCommand(deps Deps) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := req.Params.Arguments["process_name"].(string)
		input, _ := req.Params.Arguments["input"].(string)
		if name == "" || input == "" {
			return nil, errors.New("process_name and input required")
		}
		if err := deps.Supervisor.SendInput(ctx, name, []byte(input)); err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(`{"status":"sent"}`), nil
	}
}
