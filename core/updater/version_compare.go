package updater

import (
	"fmt"
	"strconv"
	"strings"
)

func CompareVersion(current string, latest string) (int, error) {
	currentParts, err := parseStableVersion(current)
	if err != nil {
		return 0, fmt.Errorf("当前版本无效: %w", err)
	}
	latestParts, err := parseStableVersion(latest)
	if err != nil {
		return 0, fmt.Errorf("最新版本无效: %w", err)
	}
	for index := 0; index < 3; index++ {
		if currentParts[index] < latestParts[index] {
			return -1, nil
		}
		if currentParts[index] > latestParts[index] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseStableVersion(value string) ([3]int, error) {
	var result [3]int
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if value == "" {
		return result, fmt.Errorf("版本号不能为空")
	}
	if strings.Contains(value, "-") || strings.Contains(value, "+") {
		return result, fmt.Errorf("暂不支持预发布或构建元数据版本: %s", value)
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("版本号必须是 x.y.z 格式: %s", value)
	}
	for index, part := range parts {
		if part == "" {
			return result, fmt.Errorf("版本号段不能为空: %s", value)
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return result, fmt.Errorf("版本号段无效: %s", part)
		}
		result[index] = number
	}
	return result, nil
}
