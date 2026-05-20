package swiggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

var errMCPFoodAuthorizerRequired = errors.New("food mcp request authorizer is required")

type MCPFoodClient struct {
	endpoint   string
	httpClient *http.Client
	authorizer RequestAuthorizer
	nextID     atomic.Int64
}

func NewMCPFoodClient(endpoint string, httpClient *http.Client, authorizer RequestAuthorizer) *MCPFoodClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &MCPFoodClient{
		endpoint:   strings.TrimSpace(endpoint),
		httpClient: httpClient,
		authorizer: authorizer,
	}
}

type foodToolEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Text    string          `json:"-"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *MCPFoodClient) callTool(ctx context.Context, name string, arguments any, target any) error {
	envelope, err := c.callToolEnvelopeHTTP(ctx, name, arguments)
	if err != nil {
		return err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		if target != nil && strings.TrimSpace(envelope.Text) != "" {
			return mapFoodTextResponse(name, envelope.Text, target)
		}
		return nil
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("map food mcp %s response: %w", name, err)
	}
	mergeFoodTextResponse(name, envelope.Text, target)
	return nil
}

func (c *MCPFoodClient) callToolEnvelope(ctx context.Context, name string, arguments any) (foodToolEnvelope, error) {
	return c.callToolEnvelopeHTTP(ctx, name, arguments)
}

func (c *MCPFoodClient) callToolEnvelopeHTTP(ctx context.Context, name string, arguments any) (foodToolEnvelope, error) {
	if c.authorizer == nil {
		return foodToolEnvelope{}, errMCPFoodAuthorizerRequired
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return foodToolEnvelope{}, errors.New("food mcp endpoint is required")
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
		return foodToolEnvelope{}, fmt.Errorf("build food mcp %s request: %w", name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return foodToolEnvelope{}, fmt.Errorf("build food mcp %s http request: %w", name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if err := c.authorizer.AuthorizeMCPRequest(ctx, req); err != nil {
		return foodToolEnvelope{}, fmt.Errorf("authorize food mcp %s request: %w", name, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return foodToolEnvelope{}, fmt.Errorf("call food mcp %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: http status %d", name, resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxMCPResponseBytes+1))
	if err != nil {
		return foodToolEnvelope{}, fmt.Errorf("read food mcp %s response: %w", name, err)
	}
	if len(respBody) > maxMCPResponseBytes {
		return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: response body exceeds %d bytes", name, maxMCPResponseBytes)
	}

	envelope, err := unwrapFoodToolEnvelope(name, respBody)
	if err != nil {
		return foodToolEnvelope{}, err
	}
	return envelope, nil
}

func unwrapFoodToolEnvelope(toolName string, body []byte) (foodToolEnvelope, error) {
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return foodToolEnvelope{}, fmt.Errorf("decode food mcp %s json-rpc response: %w", toolName, err)
	}
	if rpcResp.Error != nil {
		message := safeUpstreamMessage(rpcResp.Error.Message, "json-rpc error")
		return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: mcp error %d: %s", toolName, rpcResp.Error.Code, message)
	}

	if hasFoodToolEnvelopeShape(rpcResp.Result) {
		return decodeFoodToolEnvelope(toolName, rpcResp.Result)
	}

	var toolResult mcpToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return foodToolEnvelope{}, fmt.Errorf("decode food mcp %s tool result: %w", toolName, err)
	}
	if toolResult.IsError {
		for _, content := range toolResult.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: %s", toolName, safeUpstreamMessage(content.Text, "tool returned an error"))
			}
		}
		return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: tool returned an error", toolName)
	}
	if len(toolResult.StructuredContent) != 0 && string(toolResult.StructuredContent) != "null" {
		text := firstTextContent(toolResult.Content)
		if hasFoodToolStatusEnvelopeShape(toolResult.StructuredContent) {
			envelope, err := decodeFoodToolEnvelope(toolName, toolResult.StructuredContent)
			envelope.Text = text
			return envelope, err
		}
		if data, ok := structuredContentData(toolResult.StructuredContent); ok {
			return foodToolEnvelope{Success: true, Data: data, Text: text}, nil
		}
		return foodToolEnvelope{Success: true, Data: toolResult.StructuredContent, Text: text}, nil
	}
	for _, content := range toolResult.Content {
		if content.Type != "text" || strings.TrimSpace(content.Text) == "" {
			continue
		}
		if envelope, err := decodeFoodToolEnvelope(toolName, []byte(content.Text)); err == nil {
			return envelope, nil
		}
		return foodToolEnvelope{Success: true, Text: content.Text}, nil
	}

	return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: missing tool response content", toolName)
}

func firstTextContent(contents []mcpToolContent) string {
	for _, content := range contents {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return content.Text
		}
	}
	return ""
}

func hasFoodToolEnvelopeShape(raw json.RawMessage) bool {
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

func hasFoodToolStatusEnvelopeShape(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	_, hasSuccess := fields["success"]
	_, hasMessage := fields["message"]
	_, hasError := fields["error"]
	return hasSuccess || hasMessage || hasError
}

func structuredContentData(raw json.RawMessage) (json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	data, ok := fields["data"]
	if !ok || len(data) == 0 || string(data) == "null" {
		return nil, false
	}
	return data, true
}

func decodeFoodToolEnvelope(toolName string, raw []byte) (foodToolEnvelope, error) {
	var envelope foodToolEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return foodToolEnvelope{}, fmt.Errorf("decode food mcp %s envelope: %w", toolName, err)
	}
	if !envelope.Success {
		message := strings.TrimSpace(envelope.Message)
		if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
			message = strings.TrimSpace(envelope.Error.Message)
		}
		message = safeUpstreamMessage(message, "tool returned unsuccessful response")
		return foodToolEnvelope{}, fmt.Errorf("food mcp %s failed: %s", toolName, message)
	}
	return envelope, nil
}
