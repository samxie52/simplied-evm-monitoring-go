package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// EnvLoader 环境变量加载器
type EnvLoader struct {
	envFile string
}

// NewEnvLoader 创建环境变量加载器
func NewEnvLoader(envFile string) *EnvLoader {
	return &EnvLoader{
		envFile: envFile,
	}
}

// Load 加载环境变量到配置结构体
func (e *EnvLoader) Load(cfg *Config) error {
	// 加载.env文件
	if e.envFile != "" {
		if err := godotenv.Load(e.envFile); err != nil {
			// .env文件不存在时不报错，使用系统环境变量
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to load env file %s: %w", e.envFile, err)
			}
		}
	}

	// 使用反射填充配置结构体
	return e.fillStruct(reflect.ValueOf(cfg).Elem())
}

// fillStruct 递归填充结构体字段
func (e *EnvLoader) fillStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 跳过非导出字段
		if !field.CanSet() {
			continue
		}

		// 处理嵌套结构体
		if field.Kind() == reflect.Struct {
			if err := e.fillStruct(field); err != nil {
				return err
			}
			continue
		}

		// 获取env标签
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// 获取环境变量值
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// 根据字段类型设置值
		if err := e.setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValue 设置字段值
func (e *EnvLoader) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			// 处理time.Duration类型
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration format: %s", value)
			}
			field.SetInt(int64(duration))
		} else {
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer format: %s", value)
			}
			field.SetInt(intValue)
		}
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean format: %s", value)
		}
		field.SetBool(boolValue)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			// 处理字符串切片
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}
