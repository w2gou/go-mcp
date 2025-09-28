package tools

import (
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"go-mcp/mcp/types"
	"runtime"
	"time"
)

// CPUTool CPU 监控工具
type CPUTool struct{}

// NewCPUTool 创建新的 CPU 监控工具
func NewCPUTool() *CPUTool {
	return &CPUTool{}
}

// GetName 获取工具名称
func (ct *CPUTool) GetName() string {
	return "cpu_info"
}

// GetDescription 获取工具描述
func (ct *CPUTool) GetDescription() string {
	return "获取 CPU 使用率和详细信息"
}

// GetInputSchema 获取输入模式
func (ct *CPUTool) GetInputSchema() types.InputSchema {
	return types.InputSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"duration": {
				Type:        "string",
				Description: "监控持续时间 (1s, 5s, 10s)",
				Enum:        []string{"1s", "5s", "10s"},
				Default:     "1s",
			},
		},
	}
}

// Execute 执行 CPU 监控
func (ct *CPUTool) Execute(args map[string]interface{}) (string, error) {
	// 解析参数
	durationStr, _ := args["duration"].(string)
	if durationStr == "" {
		durationStr = "1s"
	}

	// 获取 CPU 信息
	cpuInfo, err := ct.getCPUInfo(durationStr)
	if err != nil {
		return "", fmt.Errorf("获取 CPU 信息失败: %v", err)
	}

	return ct.formatCPUInfo(cpuInfo, durationStr), nil
}

// getCPUInfo 获取 CPU 信息
func (ct *CPUTool) getCPUInfo(durationStr string) (types.CPUInfo, error) {
	var cpuInfo types.CPUInfo

	// 解析持续时间
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = time.Second
	}

	// 获取 CPU 基本信息
	cpuInfos, err := cpu.Info()
	if err != nil {
		return cpuInfo, fmt.Errorf("获取 CPU 基本信息失败: %v", err)
	}

	if len(cpuInfos) > 0 {
		cpuInfo.ModelName = cpuInfos[0].ModelName
		cpuInfo.Cores = cpuInfos[0].Cores
		cpuInfo.Frequency = cpuInfos[0].Mhz / 1000 // 转换为 GHz
	}

	cpuInfo.LogicalCores = runtime.NumCPU()

	// 获取 CPU 使用率
	cpuPercent, err := cpu.Percent(duration, true)
	if err != nil {
		return cpuInfo, fmt.Errorf("获取 CPU 使用率失败: %v", err)
	}

	// 获取总体 CPU 使用率
	totalCPU, err := cpu.Percent(duration, false)
	if err != nil {
		return cpuInfo, fmt.Errorf("获取总体 CPU 使用率失败: %v", err)
	}

	// 设置使用率数据
	cpuInfo.Usage.PerCore = cpuPercent
	if len(totalCPU) > 0 {
		cpuInfo.Usage.Total = totalCPU[0]
	}

	cpuInfo.LastUpdated = time.Now()

	return cpuInfo, nil
}

// formatCPUInfo 格式化 CPU 信息输出
func (ct *CPUTool) formatCPUInfo(cpuInfo types.CPUInfo, durationStr string) string {
	var result string

	result += "🖥️  CPU 信息\n"
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	result += fmt.Sprintf("型号: %s\n", cpuInfo.ModelName)
	result += fmt.Sprintf("核心数: %d 物理核心, %d 逻辑核心\n", cpuInfo.Cores, cpuInfo.LogicalCores)
	result += fmt.Sprintf("主频: %.2f GHz\n", cpuInfo.Frequency)

	result += fmt.Sprintf("\n📊 CPU 使用率 (监控时长: %s)\n", durationStr)
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	result += fmt.Sprintf("总体使用率: %.2f%%\n\n", cpuInfo.Usage.Total)

	result += "各核心使用率:\n"
	for i, percent := range cpuInfo.Usage.PerCore {
		result += fmt.Sprintf("  核心 %d: %.2f%%\n", i+1, percent)
	}

	result += fmt.Sprintf("\n📅 更新时间: %s\n", cpuInfo.LastUpdated.Format("2006-01-02 15:04:05"))

	return result
}

// GetCPUData 获取 CPU 数据（供其他组件使用）
func (ct *CPUTool) GetCPUData(duration time.Duration) (types.CPUInfo, error) {
	durationStr := duration.String()
	return ct.getCPUInfo(durationStr)
}

// CPUTool returns the tool definition and handler for the cpu_status tool.
//func CPUTool() (ToolDefinition, Handler) {
//	schema := JSONSchema{
//		"type":                 "object",
//		"description":          "Optional parameters (currently unused).",
//		"additionalProperties": false,
//	}
//
//	handler := func(ctx context.Context, arguments json.RawMessage) (*types.CallToolResult, error) {
//
//		cpuInfo, err := getCPUInfo()
//		if err != nil {
//			return nil, fmt.Errorf("获取 CPU 信息失败: %v", err)
//		}
//
//		return &types.CallToolResult{
//			Content: []types.ContentItem{TextContent(formatCPUInfo(cpuInfo))},
//		}, nil
//	}
//
//	return ToolDefinition{
//		Name:        "cpu_status",
//		Description: "获取 CPU 使用率和详细信息",
//		InputSchema: schema,
//	}, handler
//}

//func init() {
//	MustRegister(CPUTool())
//}
