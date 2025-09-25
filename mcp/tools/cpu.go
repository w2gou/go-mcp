package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go-mcp/mcp/types"
)

const cpuSampleWindow = 250 * time.Millisecond

// CPUTool returns the tool definition and handler for the cpu_status tool.
func CPUTool() (ToolDefinition, Handler) {
	schema := JSONSchema{
		"type":                 "object",
		"description":          "Optional parameters (currently unused).",
		"additionalProperties": false,
	}

	handler := func(ctx context.Context, arguments json.RawMessage) (*types.CallToolResult, error) {
		if err := ensureNoArguments(arguments); err != nil {
			return nil, err
		}

		loadAvg, err := readLoadAverages()
		if err != nil {
			return nil, err
		}

		usage, err := sampleCPUUsage(ctx, cpuSampleWindow)
		if err != nil {
			return nil, err
		}

		text := fmt.Sprintf(
			"CPU cores: %d\nLoad average (1m, 5m, 15m): %.2f %.2f %.2f\nSampled utilization: %.2f%% over %s",
			runtime.NumCPU(),
			loadAvg[0], loadAvg[1], loadAvg[2],
			usage*100,
			cpuSampleWindow,
		)

		return &types.CallToolResult{
			Content: []types.ContentItem{TextContent(text)},
		}, nil
	}

	return ToolDefinition{
		Name:        "cpu_status",
		Description: "Report CPU load averages and recent utilization.",
		InputSchema: schema,
	}, handler
}

func init() {
	MustRegister(CPUTool())
}

func ensureNoArguments(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if len(payload) != 0 {
		return fmt.Errorf("cpu_status does not accept arguments")
	}

	return nil
}

func readLoadAverages() ([3]float64, error) {
	var result [3]float64

	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return result, fmt.Errorf("read /proc/loadavg: %w", err)
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return result, fmt.Errorf("unexpected /proc/loadavg format")
	}

	for i := 0; i < 3; i++ {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return result, fmt.Errorf("parse load average: %w", err)
		}
		result[i] = value
	}

	return result, nil
}

func sampleCPUUsage(ctx context.Context, window time.Duration) (float64, error) {
	idle1, total1, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	if window <= 0 {
		window = cpuSampleWindow
	}

	timer := time.NewTimer(window)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-timer.C:
	}

	idle2, total2, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta == 0 {
		return 0, nil
	}

	usage := 1 - float64(idleDelta)/float64(totalDelta)
	if usage < 0 {
		usage = 0
	}
	if usage > 1 {
		usage = 1
	}

	return usage, nil
}

func readCPUTimes() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/stat: %w", err)
	}

	content := string(data)
	lineEnd := strings.IndexByte(content, '\n')
	line := content
	if lineEnd >= 0 {
		line = content[:lineEnd]
	}

	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat format")
	}

	var total uint64
	var idle uint64

	for idx, value := range fields[1:] {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parse /proc/stat field: %w", err)
		}

		total += parsed
		if idx == 3 || idx == 4 {
			idle += parsed
		}
	}

	return idle, total, nil
}
