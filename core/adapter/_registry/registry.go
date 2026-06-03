package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	registryMu  sync.RWMutex
	descriptors = make(map[string]Descriptor)
)

// Register 注册一个平台适配器描述。
func Register(desc Descriptor) {
	desc.Platform = strings.TrimSpace(desc.Platform)
	desc.DisplayName = strings.TrimSpace(desc.DisplayName)
	if desc.Platform == "" {
		panic("适配器 platform 不能为空")
	}
	if desc.DisplayName == "" {
		desc.DisplayName = desc.Platform
	}
	if desc.ParseConfig == nil {
		panic(fmt.Sprintf("适配器 %s 缺少配置解析器", desc.Platform))
	}
	if desc.NewAdapter == nil {
		panic(fmt.Sprintf("适配器 %s 缺少构造器", desc.Platform))
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := descriptors[desc.Platform]; exists {
		panic(fmt.Sprintf("适配器 platform 重复注册: %s", desc.Platform))
	}
	descriptors[desc.Platform] = cloneDescriptor(desc)
}

// Get 按平台名获取适配器描述。
func Get(platform string) (Descriptor, bool) {
	platform = strings.TrimSpace(platform)
	registryMu.RLock()
	defer registryMu.RUnlock()
	desc, ok := descriptors[platform]
	if !ok {
		return Descriptor{}, false
	}
	return cloneDescriptor(desc), true
}

// List 返回全部已注册适配器描述，顺序按 platform 稳定排序。
func List() []Descriptor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	items := make([]Descriptor, 0, len(descriptors))
	for _, desc := range descriptors {
		items = append(items, cloneDescriptor(desc))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Platform < items[j].Platform
	})
	return items
}

func cloneDescriptor(desc Descriptor) Descriptor {
	cloned := desc
	if desc.ConfigSchema != nil {
		cloned.ConfigSchema = append([]ConfigField(nil), desc.ConfigSchema...)
	}
	return cloned
}
