package utils

import (
	"fmt"
	"sync"
)

// executionGuard 是一个包级别的私有变量，确保全局唯一
var executionGuard = &ExecutionGuard{
	m: make(map[string]struct{}),
}

// ExecutionGuard 用于确保对于给定的 key，一个操作在同一时间只会被执行一次。
type ExecutionGuard struct {
	mu sync.Mutex
	m  map[string]struct{}
}

// Do 是一个包级别的公开函数，用于执行被守护的操作。
// 它会使用全局的 executionGuard 实例。
func Do(key string, fn func() error) error {
	return executionGuard.do(key, fn)
}

// do 是实际的执行逻辑。
func (g *ExecutionGuard) do(key string, fn func() error) error {
	g.mu.Lock()
	if _, exists := g.m[key]; exists {
		g.mu.Unlock()
		fmt.Printf("DEBUG: Guard for key '%s' is active, skipping.\n", key) // 增加调试日志
		return nil
	}

	g.m[key] = struct{}{}
	g.mu.Unlock()

	fmt.Printf("DEBUG: Acquiring guard for key '%s'.\n", key) // 增加调试日志
	err := fn()

	g.mu.Lock()
	delete(g.m, key)
	fmt.Printf("DEBUG: Releasing guard for key '%s'.\n", key) // 增加调试日志
	g.mu.Unlock()

	return err
}
