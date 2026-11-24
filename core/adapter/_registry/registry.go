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
	if err := RegisterDynamic(desc); err != nil {
		panic(err)
	}
}

// RegisterDynamic registers a descriptor loaded at runtime.
func RegisterDynamic(desc Descriptor) error {
	desc.Platform = strings.TrimSpace(desc.Platform)
	desc.DisplayName = strings.TrimSpace(desc.DisplayName)
	if desc.Platform == "" {
		return fmt.Errorf("适配器 platform 不能为空")
	}
	if desc.DisplayName == "" {
		desc.DisplayName = desc.Platform
	}
	if desc.ParseConfig == nil {
		return fmt.Errorf("适配器 %s 缺少配置解析器", desc.Platform)
	}
	if desc.NewAdapter == nil {
		return fmt.Errorf("适配器 %s 缺少构造器", desc.Platform)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := descriptors[desc.Platform]; exists {
		return fmt.Errorf("适配器 platform 重复注册: %s", desc.Platform)
	}
	descriptors[desc.Platform] = cloneDescriptor(desc)
	return nil
}

func ReplaceCustom(desc Descriptor) error {
	desc.Platform = strings.TrimSpace(desc.Platform)
	desc.DisplayName = strings.TrimSpace(desc.DisplayName)
	if !desc.Custom || desc.Platform == "" || desc.ParseConfig == nil || desc.NewAdapter == nil {
		return fmt.Errorf("自定义适配器定义无效")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if existing, ok := descriptors[desc.Platform]; ok && !existing.Custom {
		return fmt.Errorf("不能替换 Go 内置适配器: %s", desc.Platform)
	}
	descriptors[desc.Platform] = cloneDescriptor(desc)
	return nil
}

func UnregisterCustom(platform string) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	desc, ok := descriptors[strings.TrimSpace(platform)]
	if !ok {
		return nil
	}
	if !desc.Custom {
		return fmt.Errorf("不能删除 Go 内置适配器: %s", platform)
	}
	delete(descriptors, strings.TrimSpace(platform))
	return nil
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
	if desc.DeclaredCapabilities != nil {
		cloned.DeclaredCapabilities = make([]string, len(desc.DeclaredCapabilities))
		copy(cloned.DeclaredCapabilities, desc.DeclaredCapabilities)
	}
	if desc.ConfigSchema != nil {
		cloned.ConfigSchema = append([]ConfigField(nil), desc.ConfigSchema...)
		for index := range cloned.ConfigSchema {
			if desc.ConfigSchema[index].Options != nil {
				cloned.ConfigSchema[index].Options = append([]ConfigOption(nil), desc.ConfigSchema[index].Options...)
			}
		}
	}
	return cloned
}
