package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"miniclaw-go/internal/config"
	"miniclaw-go/internal/core"
)

type Client struct {
	cfg     config.MCPServerConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	pending sync.Map
	nextID  int64
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewClient(cfg config.MCPServerConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Start(ctx context.Context) error {
	cmd := exec.Command(c.cfg.Command, c.cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range c.cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mcp server %s: %w", c.cfg.Name, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.reader = bufio.NewReader(stdout)
	go c.readLoop()

	if err := c.initialize(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}

func (c *Client) ListTools(ctx context.Context) ([]core.ToolDescriptor, error) {
	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", map[string]any{}, &result); err != nil {
		return nil, err
	}

	var tools []core.ToolDescriptor
	for _, tool := range result.Tools {
		tools = append(tools, core.ToolDescriptor{
			Name:        "mcp." + c.cfg.Name + "." + tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Source:      "mcp",
		})
	}
	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, registeredName string, args json.RawMessage) (string, error) {
	toolName := strings.TrimPrefix(registeredName, "mcp."+c.cfg.Name+".")
	var payload map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &payload); err != nil {
			return "", fmt.Errorf("decode mcp args: %w", err)
		}
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent map[string]any `json:"structuredContent"`
	}
	if err := c.call(ctx, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": payload,
	}, &result); err != nil {
		return "", err
	}

	var parts []string
	for _, item := range result.Content {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	if len(parts) == 0 && len(result.StructuredContent) > 0 {
		data, _ := json.MarshalIndent(result.StructuredContent, "", "  ")
		parts = append(parts, string(data))
	}
	if len(parts) == 0 {
		parts = append(parts, "mcp tool completed with no text response")
	}
	return strings.Join(parts, "\n"), nil
}

func (c *Client) initialize(ctx context.Context) error {
	var response map[string]any
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "miniclaw-go",
			"version": "0.1.0",
		},
	}, &response); err != nil {
		return fmt.Errorf("initialize mcp server %s: %w", c.cfg.Name, err)
	}
	return c.notify("notifications/initialized", map[string]any{})
}

func (c *Client) notify(method string, params interface{}) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.writeFrame(req)
}

func (c *Client) call(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := atomic.AddInt64(&c.nextID, 1)
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan rpcResponse, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	if err := c.writeFrame(req); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(resp.Result, out)
	}
}

func (c *Client) writeFrame(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		body := scanner.Bytes()
		if len(strings.TrimSpace(string(body))) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if chValue, ok := c.pending.Load(resp.ID); ok {
			ch := chValue.(chan rpcResponse)
			ch <- resp
		}
	}
}
