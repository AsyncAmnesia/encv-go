package reader

import (
	"os"
	"sync"
)

// FileHandlePool 管理基于路径的文件句柄，使用引用计数来避免重复打开和过早关闭。
// 这是解决高并发场景下 "Too many open files" 问题的核心。
var globalFileHandlePool = &fileHandlePool{
	holds: make(map[string]*fileHandleHold),
}

type fileHandlePool struct {
	mu    sync.Mutex
	holds map[string]*fileHandleHold
}

type fileHandleHold struct {
	file  *os.File
	count int64 // 引用计数
}

// Get 从池中获取一个文件句柄。如果不存在，则打开它并增加引用计数。
func (p *fileHandlePool) Get(path string) (*os.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	hold, exists := p.holds[path]
	if !exists {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		hold = &fileHandleHold{file: file, count: 0}
		p.holds[path] = hold
	}
	hold.count++
	return hold.file, nil
}

// Put 归还一个文件句柄。当引用计数归零时，才真正关闭文件。
func (p *fileHandlePool) Put(file *os.File) error {
	if file == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	path := file.Name()
	hold, exists := p.holds[path]
	if !exists {
		// 异常情况，直接关闭
		return file.Close()
	}

	hold.count--
	if hold.count <= 0 {
		delete(p.holds, path)
		return file.Close()
	}
	return nil
}
