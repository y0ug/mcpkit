package mcpkit

import (
	"context"
	"encoding/json"
	"log/slog"
	"os/exec"
	"testing"
	"time"
)

func TestClientIntegration(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := slog.Default()

	// Create client connected to Dockerized MCP server
	client, err := NewClient(
		ctx,
		logger,
		"docker",
		"run", "--rm", "-i", "mcp/time",
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test initialization sequence
	t.Run("Initialize", func(t *testing.T) {
		serverInfo, err := client.Initialize(ctx)
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if serverInfo.ServerInfo.Name == "" {
			t.Error("Expected server name in initialization response")
		}
		t.Logf("Connected to server: %s v%s",
			serverInfo.ServerInfo.Name,
			serverInfo.ServerInfo.Version,
		)
	})

	// Test tool listing functionality
	t.Run("ListTools", func(t *testing.T) {
		tools, _, err := client.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) == 0 {
			t.Error("Expected non-empty list of tools")
		}

		for _, tool := range tools {
			var desc string
			if tool.Description != nil {
				desc = *tool.Description
			}
			t.Logf("Found tool: %s - %s", tool.Name, desc)
			if tool.InputSchema.Type == "" {
				t.Errorf("Tool %s has invalid input schema", tool.Name)
			}
			t.Logf("Input schema:\n%v", tool.InputSchema)
		}
	})

	// Test basic ping functionality
	t.Run("Ping", func(t *testing.T) {
		if err := client.Ping(ctx); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}

func TestClientIntegrationServer(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := slog.Default()

	// Create client connected to Dockerized MCP server
	client, err := NewClient(
		ctx,
		logger,
		"go",
		"run", "./cmd/mcp-time/main.go",
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test initialization sequence
	t.Run("Initialize", func(t *testing.T) {
		serverInfo, err := client.Initialize(ctx)
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if serverInfo.ServerInfo.Name == "" {
			t.Error("Expected server name in initialization response")
		}
		t.Logf("Connected to server: %s v%s",
			serverInfo.ServerInfo.Name,
			serverInfo.ServerInfo.Version,
		)
	})

	// Test tool listing functionality
	t.Run("ListTools", func(t *testing.T) {
		tools, _, err := client.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) == 0 {
			t.Error("Expected non-empty list of tools")
		}

		for _, tool := range tools {
			var desc string
			if tool.Description != nil {
				desc = *tool.Description
			}
			t.Logf("Found tool: %s - %s", tool.Name, desc)
			if tool.InputSchema.Type == "" {
				t.Errorf("Tool %s has invalid input schema", tool.Name)
			}
			t.Logf("Input schema:\n%v", tool.InputSchema)
		}
	})

	// Test basic ping functionality
	t.Run("Ping", func(t *testing.T) {
		if err := client.Ping(ctx); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

func TestClientIntegrationCall2(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := slog.Default()

	client, err := NewClient(
		ctx,
		logger,
		"docker",
		"run", "--rm", "-i", "mcp/time",
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	t.Run("GetCurrentTime", func(t *testing.T) {
		args := map[string]interface{}{
			"timezone": "UTC",
		}

		result, err := client.CallTool(ctx, "get_current_time", args)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		// Parse the text content as JSON
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(result.Content[0].(map[string]interface{})["text"].(string)), &response); err != nil {
			t.Fatalf("Failed to parse response JSON: %v", err)
		}

		if _, ok := response["datetime"]; !ok {
			t.Error("Missing datetime in response")
		}

		if tz, ok := response["timezone"].(string); !ok || tz != "UTC" {
			t.Errorf("Unexpected timezone in response: %v", response["timezone"])
		}
	})

	t.Run("ConvertTime", func(t *testing.T) {
		args := map[string]interface{}{
			"time":            "06:00",
			"source_timezone": "UTC",
			"target_timezone": "America/New_York",
		}

		result, err := client.CallTool(ctx, "convert_time", args)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		// Handle error results
		if result.IsError != nil && *result.IsError {
			t.Log("ToolCall Error result: ", result)
		}

		// Parse the text content as JSON
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(result.Content[0].(map[string]interface{})["text"].(string)), &response); err != nil {
			t.Fatalf("Failed to parse response JSON: %v", err)
		}
		t.Log("Content: ", result.Content)
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		args := map[string]interface{}{
			"timezone": "Invalid/Timezone",
		}

		result, err := client.CallTool(ctx, "get_current_time", args)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		if result.IsError != nil && *result.IsError {
			t.Log("ToolCall Error result: ", result)
		} else {
			t.Fatal("Expected error result for invalid timezone")
		}
	})
}
