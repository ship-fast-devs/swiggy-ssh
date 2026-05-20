package swiggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
)

const mcpJSONRPCVersion = "2.0"

const maxMCPResponseBytes = 4 << 20

var errMCPRequestAuthorizerRequired = errors.New("instamart mcp request authorizer is required")

var longDigitSequence = regexp.MustCompile(`\d{6,}`)

// RequestAuthorizer is supplied by the auth/MCP-session boundary. The provider
// does not know how tokens or MCP sessions are obtained.
type RequestAuthorizer interface {
	AuthorizeMCPRequest(ctx context.Context, req *http.Request) error
}

type MCPInstamartClient struct {
	endpoint   string
	httpClient *http.Client
	authorizer RequestAuthorizer
	nextID     atomic.Int64
}

func NewMCPInstamartClient(endpoint string, httpClient *http.Client, authorizer RequestAuthorizer) *MCPInstamartClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &MCPInstamartClient{
		endpoint:   strings.TrimSpace(endpoint),
		httpClient: httpClient,
		authorizer: authorizer,
	}
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int64         `json:"id"`
	Method  string        `json:"method"`
	Params  mcpCallParams `json:"params"`
}

type mcpCallParams struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolResult struct {
	Content           []mcpToolContent `json:"content"`
	StructuredContent json.RawMessage  `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}

type mcpToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type instamartToolEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *MCPInstamartClient) callTool(ctx context.Context, name string, arguments any, target any) error {
	envelope, err := c.callToolEnvelopeHTTP(ctx, name, arguments)
	if err != nil {
		return err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("map instamart mcp %s response: %w", name, err)
	}
	return nil
}

func (c *MCPInstamartClient) callToolEnvelopeHTTP(ctx context.Context, name string, arguments any) (instamartToolEnvelope, error) {
	if c.authorizer == nil {
		return instamartToolEnvelope{}, errMCPAuthorizerRequired()
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return instamartToolEnvelope{}, errors.New("instamart mcp endpoint is required")
	}

	body, err := json.Marshal(jsonRPCRequest{
		JSONRPC: mcpJSONRPCVersion,
		ID:      c.nextID.Add(1),
		Method:  "tools/call",
		Params: mcpCallParams{
			Name:      name,
			Arguments: arguments,
		},
	})
	if err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("build instamart mcp %s request: %w", name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("build instamart mcp %s http request: %w", name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if err := c.authorizer.AuthorizeMCPRequest(ctx, req); err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("authorize instamart mcp %s request: %w", name, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("call instamart mcp %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: http status %d", name, resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxMCPResponseBytes+1))
	if err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("read instamart mcp %s response: %w", name, err)
	}
	if len(respBody) > maxMCPResponseBytes {
		return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: response body exceeds %d bytes", name, maxMCPResponseBytes)
	}

	envelope, err := unwrapInstamartToolEnvelope(name, respBody)
	if err != nil {
		return instamartToolEnvelope{}, err
	}
	return envelope, nil
}

func errMCPAuthorizerRequired() error {
	return errMCPRequestAuthorizerRequired
}

func unwrapInstamartToolEnvelope(toolName string, body []byte) (instamartToolEnvelope, error) {
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("decode instamart mcp %s json-rpc response: %w", toolName, err)
	}
	if rpcResp.Error != nil {
		message := safeUpstreamMessage(rpcResp.Error.Message, "json-rpc error")
		return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: mcp error %d: %s", toolName, rpcResp.Error.Code, message)
	}

	if hasToolEnvelopeShape(rpcResp.Result) {
		return decodeInstamartToolEnvelope(toolName, rpcResp.Result)
	}

	var toolResult mcpToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("decode instamart mcp %s tool result: %w", toolName, err)
	}
	for _, content := range toolResult.Content {
		if content.Type != "text" || strings.TrimSpace(content.Text) == "" {
			continue
		}
		if envelope, err := decodeInstamartToolEnvelope(toolName, []byte(content.Text)); err == nil {
			return envelope, nil
		}
	}

	if toolResult.IsError {
		return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: tool returned an error", toolName)
	}
	return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: missing tool response content", toolName)
}

func hasToolEnvelopeShape(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	_, hasSuccess := fields["success"]
	_, hasData := fields["data"]
	_, hasMessage := fields["message"]
	_, hasError := fields["error"]
	return hasSuccess || hasData || hasMessage || hasError
}

func decodeInstamartToolEnvelope(toolName string, raw []byte) (instamartToolEnvelope, error) {
	var envelope instamartToolEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return instamartToolEnvelope{}, fmt.Errorf("decode instamart mcp %s envelope: %w", toolName, err)
	}
	if !envelope.Success {
		message := strings.TrimSpace(envelope.Message)
		if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
			message = strings.TrimSpace(envelope.Error.Message)
		}
		message = safeUpstreamMessage(message, "tool returned unsuccessful response")
		return instamartToolEnvelope{}, fmt.Errorf("instamart mcp %s failed: %s", toolName, message)
	}
	return envelope, nil
}

func safeUpstreamMessage(message, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	lower := strings.ToLower(message)
	for _, sensitiveTerm := range []string{"token", "authorization", "bearer", "phone", "mobile", "address", "lat", "lng", "orderid", "order id"} {
		if strings.Contains(lower, sensitiveTerm) {
			return fallback
		}
	}
	message = longDigitSequence.ReplaceAllString(message, "[redacted]")
	runes := []rune(message)
	if len(runes) > 160 {
		message = string(runes[:160])
	}
	return message
}
