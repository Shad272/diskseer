// Package platform detects capabilities without changing system settings.
package platform

import "runtime"

type Capabilities struct {
	OS                 string `json:"os"`
	Architecture       string `json:"architecture"`
	NativeArchitecture string `json:"nativeArchitecture"`
	CPUs               int    `json:"logicalCPUs"`
	PhysicalMemory     uint64 `json:"physicalMemory"`
	AvailableMemory    uint64 `json:"availableMemory"`
	MemoryLoad         uint32 `json:"memoryLoadPercent"`
	Major              uint32 `json:"windowsMajor"`
	Minor              uint32 `json:"windowsMinor"`
	Build              uint32 `json:"windowsBuild"`
	Console            bool   `json:"console"`
	InputConsole       bool   `json:"inputConsole"`
	OwnConsole         bool   `json:"ownConsole"`
	InTerminal         bool   `json:"inWindowsTerminal"`
	VTAvailable        bool   `json:"virtualTerminalAPI"`
	SSE2               bool   `json:"sse2"`
}

func base() Capabilities {
	return Capabilities{OS: runtime.GOOS, Architecture: runtime.GOARCH, NativeArchitecture: runtime.GOARCH, CPUs: runtime.NumCPU()}
}

func (c Capabilities) CanUseTerminal() bool {
	return c.OS == "windows" && c.Major >= 10 && c.Build >= 19041 && c.VTAvailable
}
