package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"miniclaw-go/internal/core"
)

type Handler func(ctx context.Context, args json.RawMessage) (string, error)

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	descs    map[string]core.ToolDescriptor
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: map[string]Handler{},
		descs:    map[string]core.ToolDescriptor{},
	}
}

func (r *Registry) Register(desc core.ToolDescriptor, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.descs[desc.Name] = desc
	r.handlers[desc.Name] = handler
}

func (r *Registry) List() []core.ToolDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]core.ToolDescriptor, 0, len(r.descs))
	for _, desc := range r.descs {
		list = append(list, desc)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (r *Registry) Execute(ctx context.Context, call core.ToolCall) core.ToolResult {
	r.mu.RLock()
	handler, ok := r.handlers[call.Name]
	r.mu.RUnlock()

	start := time.Now()
	result := core.ToolResult{
		ToolCallID: call.ID,
		ToolName:   call.Name,
	}
	if !ok {
		result.Error = fmt.Sprintf("tool not found: %s", call.Name)
		result.DurationMS = time.Since(start).Milliseconds()
		return result
	}

	content, err := handler(ctx, call.Arguments)
	result.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Content = content
	}
	return result
}
