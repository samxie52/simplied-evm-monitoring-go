// Package models 定义系统中所有的数据模型
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
)

// Model 通用模型接口
type Model interface {
	// TableName 返回数据库表名
	TableName() string
	// Validate 验证模型数据
	Validate() error
	// ToJSON 序列化为 JSON
	ToJSON() ([]byte, error)
	// FromJSON 从 JSON 反序列化
	FromJSON(data []byte) error
}

// BaseModel 基础模型结构，包含通用字段
type BaseModel struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// Validate 基础验证方法
func (bm *BaseModel) Validate() error {
	validate := validator.New()
	return validate.Struct(bm)
}

// ToJSON 基础序列化方法
func (bm *BaseModel) ToJSON() ([]byte, error) {
	return json.Marshal(bm)
}

// FromJSON 基础反序列化方法
func (bm *BaseModel) FromJSON(data []byte) error {
	return json.Unmarshal(data, bm)
}

// GetID 获取模型 ID
func (bm *BaseModel) GetID() uint64 {
	return bm.ID
}

// GetCreatedAt 获取创建时间
func (bm *BaseModel) GetCreatedAt() time.Time {
	return bm.CreatedAt
}

// GetUpdatedAt 获取更新时间
func (bm *BaseModel) GetUpdatedAt() time.Time {
	return bm.UpdatedAt
}

// IsNew 检查是否为新记录
func (bm *BaseModel) IsNew() bool {
	return bm.ID == 0
}

// Touch 更新时间戳
func (bm *BaseModel) Touch() {
	bm.UpdatedAt = time.Now()
}

// ValidateStruct 通用结构体验证函数
func ValidateStruct(s interface{}) error {
	validate := validator.New()
	return validate.Struct(s)
}

// ToJSONString 将结构体转换为 JSON 字符串
func ToJSONString(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSONString 从 JSON 字符串解析到结构体
func FromJSONString(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// PaginationParams 分页参数
type PaginationParams struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=100"`
	OrderBy  string `json:"order_by"`
	Order    string `json:"order" validate:"omitempty,oneof=asc desc"`
}

// GetOffset 计算偏移量
func (p *PaginationParams) GetOffset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	return (p.Page - 1) * p.PageSize
}

// GetLimit 获取限制数量
func (p *PaginationParams) GetLimit() int {
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	return p.PageSize
}

// GetOrderClause 获取排序子句
func (p *PaginationParams) GetOrderClause() string {
	if p.OrderBy == "" {
		p.OrderBy = "id"
	}
	if p.Order == "" {
		p.Order = "desc"
	}
	return p.OrderBy + " " + p.Order
}

// PaginationResult 分页结果
type PaginationResult struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

// NewPaginationResult 创建分页结果
func NewPaginationResult(data interface{}, total int64, params *PaginationParams) *PaginationResult {
	totalPages := int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	
	return &PaginationResult{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
		HasNext:    params.Page < totalPages,
		HasPrev:    params.Page > 1,
	}
}

// FilterParams 通用过滤参数
type FilterParams struct {
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Status    string     `json:"status"`
	Type      string     `json:"type"`
	UserID    *uint64    `json:"user_id"`
}

// HasTimeFilter 检查是否有时间过滤条件
func (f *FilterParams) HasTimeFilter() bool {
	return f.StartTime != nil || f.EndTime != nil
}

// GetTimeRange 获取时间范围
func (f *FilterParams) GetTimeRange() (start, end time.Time) {
	if f.StartTime != nil {
		start = *f.StartTime
	}
	if f.EndTime != nil {
		end = *f.EndTime
	} else {
		end = time.Now()
	}
	return start, end
}

// Response 通用 API 响应结构
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}, message ...string) *Response {
	resp := &Response{
		Success: true,
		Data:    data,
	}
	if len(message) > 0 {
		resp.Message = message[0]
	}
	return resp
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(err string, code ...int) *Response {
	resp := &Response{
		Success: false,
		Error:   err,
	}
	if len(code) > 0 {
		resp.Code = code[0]
	}
	return resp
}

// ToJSON 响应序列化
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// Constants 常用常量
const (
	// 默认分页大小
	DefaultPageSize = 20
	MaxPageSize     = 100
	
	// 时间格式
	TimeFormat = "2006-01-02 15:04:05"
	DateFormat = "2006-01-02"
	
	// 以太坊相关常量
	WeiPerEther = 1e18
	WeiPerGwei  = 1e9
	
	// 交易阈值
	LargeTransactionThreshold = 100.0 // ETH
	
	// 状态常量
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusDeleted  = "deleted"
)

// 错误常量
var (
	ErrRecordNotFound = errors.New("record not found")
	ErrInvalidData    = errors.New("invalid data")
	ErrDuplicateKey   = errors.New("duplicate key")
	ErrValidation     = errors.New("validation failed")
)

// 订阅模板常量
var SubscriptionTemplates = map[SubscriptionType]string{
	SubTypeBlock:       "新区块通知: 区块 #{{.BlockNumber}} 已生成",
	SubTypeTransaction: "交易通知: {{.Hash}} - {{.Value}} ETH",
	SubTypeGasPrice:    "Gas 价格通知: 当前价格 {{.GasPrice}} Gwei",
	SubTypeAlert:       "告警通知: {{.Title}} - {{.Message}}",
}
