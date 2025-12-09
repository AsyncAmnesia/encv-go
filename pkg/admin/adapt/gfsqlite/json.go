package gfsqlite

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
)

// ProcessJsonField 处理 JSON 字符串字段
// 将数据库中的 JSON 字符串转换为对应的数据结构（map[string]interface{} 或 []interface{}）
// 主要用于兼容 SQLite（TEXT 类型存储 JSON）和 MySQL（JSON 类型）的差异
func ProcessJsonField(ctx context.Context, record gdb.Record, fieldName string) {
	if value := record[fieldName]; value != nil {
		gv := gvar.New(value)
		if !gv.IsEmpty() {
			strVal := gv.String()
			// 只处理 JSON 格式的字符串
			if strVal != "" && strings.HasPrefix(strVal, "{") && strings.HasSuffix(strVal, "}") {
				var jsonData map[string]interface{}
				if err := gjson.DecodeTo(strVal, &jsonData); err == nil {
					record[fieldName] = gvar.New(jsonData)
				} else {
					// 解析失败，设置为 nil
					g.Log().Debugf(ctx, "解析 %s JSON 失败: %v", fieldName, err)
					record[fieldName] = nil
				}
			} else if strVal != "" && strings.HasPrefix(strVal, "[") && strings.HasSuffix(strVal, "]") {
				// 处理数组格式的 JSON
				var jsonData []interface{}
				if err := gjson.DecodeTo(strVal, &jsonData); err == nil {
					record[fieldName] = gvar.New(jsonData)
				} else {
					g.Log().Debugf(ctx, "解析 %s JSON 数组失败: %v", fieldName, err)
					record[fieldName] = nil
				}
			} else {
				// 不是 JSON 格式，设置为 nil
				record[fieldName] = nil
			}
		} else {
			record[fieldName] = nil
		}
	} else {
		record[fieldName] = nil
	}
}

// ProcessJsonFieldForInterface 处理任意 interface{} 中的 JSON 字符串字段
// 这个函数更通用，可以处理任何包含指定字段的 map
func ProcessJsonFieldForInterface(ctx context.Context, data map[string]interface{}, fieldName string) {
	if value, ok := data[fieldName]; ok && value != nil {
		gv := gvar.New(value)
		if !gv.IsEmpty() {
			strVal := gv.String()
			if strVal != "" && strings.HasPrefix(strVal, "{") && strings.HasSuffix(strVal, "}") {
				var jsonData map[string]interface{}
				if err := gjson.DecodeTo(strVal, &jsonData); err == nil {
					data[fieldName] = jsonData
				} else {
					g.Log().Debugf(ctx, "解析 %s JSON 失败: %v", fieldName, err)
					data[fieldName] = nil
				}
			} else if strVal != "" && strings.HasPrefix(strVal, "[") && strings.HasSuffix(strVal, "]") {
				var jsonData []interface{}
				if err := gjson.DecodeTo(strVal, &jsonData); err == nil {
					data[fieldName] = jsonData
				} else {
					g.Log().Debugf(ctx, "解析 %s JSON 数组失败: %v", fieldName, err)
					data[fieldName] = nil
				}
			} else {
				data[fieldName] = nil
			}
		} else {
			data[fieldName] = nil
		}
	} else {
		data[fieldName] = nil
	}
}
