package models

import (
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/go-playground/validator/v10"
)

// Transaction 交易数据模型
type Transaction struct {
	BaseModel
	
	// 交易基本信息
	Hash        string    `json:"hash" gorm:"uniqueIndex;size:66;not null" validate:"required,len=66"`
	BlockNumber uint64    `json:"block_number" gorm:"index;not null" validate:"required,min=0"`
	BlockHash   string    `json:"block_hash" gorm:"size:66;index" validate:"len=66"`
	Index       uint32    `json:"transaction_index" gorm:"index" validate:"min=0"`
	
	// 发送方和接收方
	From        string    `json:"from_address" gorm:"size:42;index;not null" validate:"required,len=42"`
	To          *string   `json:"to_address" gorm:"size:42;index"` // 合约创建时为 nil
	
	// 交易金额和数据
	Value       string    `json:"value" gorm:"type:varchar(78);not null" validate:"required"` // Wei 单位
	Input       string    `json:"input" gorm:"type:text"`
	
	// Gas 相关
	Gas         uint64    `json:"gas" validate:"required,min=0"`
	GasUsed     *uint64   `json:"gas_used" validate:"omitempty,min=0"`
	GasPrice    *string   `json:"gas_price" gorm:"type:varchar(78)"` // Legacy 交易
	
	// EIP-1559 字段
	MaxFeePerGas         *string `json:"max_fee_per_gas" gorm:"type:varchar(78)"`
	MaxPriorityFeePerGas *string `json:"max_priority_fee_per_gas" gorm:"type:varchar(78)"`
	
	// 交易类型和状态
	Type        TransactionType   `json:"type" gorm:"type:varchar(20);index;not null" validate:"required"`
	Status      TransactionStatus `json:"status" gorm:"type:varchar(20);index;not null" validate:"required"`
	
	// 签名信息
	Nonce       uint64    `json:"nonce" validate:"min=0"`
	V           string    `json:"v" gorm:"size:10"`
	R           string    `json:"r" gorm:"size:66" validate:"len=66"`
	S           string    `json:"s" gorm:"size:66" validate:"len=66"`
	
	// 执行结果
	CumulativeGasUsed *uint64 `json:"cumulative_gas_used"`
	EffectiveGasPrice *string `json:"effective_gas_price" gorm:"type:varchar(78)"`
	
	// 合约相关
	ContractAddress *string `json:"contract_address" gorm:"size:42"` // 合约创建时的地址
	
	// 日志和事件
	LogsCount   uint32 `json:"logs_count" validate:"min=0"`
	LogsBloom   string `json:"logs_bloom" gorm:"type:text"`
	
	// 时间戳
	Timestamp   time.Time `json:"timestamp" gorm:"index"`
	
	// 关联关系
	Block       Block            `json:"block,omitempty"`
	Logs        []TransactionLog `json:"logs,omitempty"`
	
	// 计算字段（不存储到数据库）
	ValueInEther    float64 `json:"value_in_ether" gorm:"-"`
	GasCostInEther  float64 `json:"gas_cost_in_ether" gorm:"-"`
	IsContractCall  bool    `json:"is_contract_call" gorm:"-"`
	IsLargeValue    bool    `json:"is_large_value" gorm:"-"`
}

// TransactionLog 交易日志模型
type TransactionLog struct {
	BaseModel
	
	TransactionHash string    `json:"transaction_hash" gorm:"size:66;index;not null"`
	LogIndex        uint32    `json:"log_index" gorm:"index"`
	Address         string    `json:"address" gorm:"size:42;index"`
	Topics          string    `json:"topics" gorm:"type:text"` // JSON 数组
	Data            string    `json:"data" gorm:"type:text"`
	BlockNumber     uint64    `json:"block_number" gorm:"index"`
	Removed         bool      `json:"removed"`
	
	// 关联关系
	Transaction Transaction `json:"transaction,omitempty"`
	Block       Block       `json:"block,omitempty"`
}

// TableName 指定表名
func (Transaction) TableName() string {
	return "transactions"
}

func (TransactionLog) TableName() string {
	return "transaction_logs"
}

// BeforeSave 保存前的钩子函数
func (t *Transaction) BeforeSave() error {
	// 计算以太币值
	if value, ok := new(big.Int).SetString(t.Value, 10); ok {
		etherValue := new(big.Float).SetInt(value)
		etherValue.Quo(etherValue, big.NewFloat(WeiPerEther))
		t.ValueInEther, _ = etherValue.Float64()
	}
	
	// 计算 Gas 费用
	t.GasCostInEther = t.GetGasCostInEther()
	
	// 判断是否为合约调用
	t.IsContractCall = t.To != nil && len(t.Input) > 2
	
	// 判断是否为大额交易 (>= 100 ETH)
	t.IsLargeValue = t.ValueInEther >= LargeTransactionThreshold
	
	return nil
}

// Validate 验证交易数据
func (t *Transaction) Validate() error {
	validate := validator.New()
	if err := validate.Struct(t); err != nil {
		return err
	}
	
	// 验证哈希格式
	if len(t.Hash) != 66 || t.Hash[:2] != "0x" {
		return errors.New("invalid transaction hash format")
	}
	
	// 验证地址格式
	if len(t.From) != 42 || t.From[:2] != "0x" {
		return errors.New("invalid from address format")
	}
	
	if t.To != nil && (len(*t.To) != 42 || (*t.To)[:2] != "0x") {
		return errors.New("invalid to address format")
	}
	
	// 验证交易类型
	if !t.Type.IsValid() {
		return errors.New("invalid transaction type")
	}
	
	// 验证交易状态
	if !t.Status.IsValid() {
		return errors.New("invalid transaction status")
	}
	
	return nil
}

// GetValueInEther 获取交易金额（以太币）
func (t *Transaction) GetValueInEther() float64 {
	if value, ok := new(big.Int).SetString(t.Value, 10); ok {
		etherValue := new(big.Float).SetInt(value)
		etherValue.Quo(etherValue, big.NewFloat(WeiPerEther))
		result, _ := etherValue.Float64()
		return result
	}
	return 0
}

// GetGasCostInEther 计算 Gas 费用（以太币）
func (t *Transaction) GetGasCostInEther() float64 {
	if t.GasUsed == nil {
		return 0
	}
	
	var gasPrice *big.Int
	
	// 根据交易类型获取 Gas 价格
	switch t.Type {
	case TxTypeLegacy, TxTypeAccessList:
		if t.GasPrice == nil {
			return 0
		}
		var ok bool
		gasPrice, ok = new(big.Int).SetString(*t.GasPrice, 10)
		if !ok {
			return 0
		}
	case TxTypeDynamicFee:
		if t.EffectiveGasPrice == nil {
			return 0
		}
		var ok bool
		gasPrice, ok = new(big.Int).SetString(*t.EffectiveGasPrice, 10)
		if !ok {
			return 0
		}
	default:
		return 0
	}
	
	gasUsed := new(big.Int).SetUint64(*t.GasUsed)
	gasCost := new(big.Int).Mul(gasUsed, gasPrice)
	etherCost := new(big.Float).SetInt(gasCost)
	etherCost.Quo(etherCost, big.NewFloat(WeiPerEther))
	
	result, _ := etherCost.Float64()
	return result
}

// IsSuccessful 检查交易是否成功
func (t *Transaction) IsSuccessful() bool {
	return t.Status == TxStatusSuccess
}

// IsContractCreation 检查是否为合约创建交易
func (t *Transaction) IsContractCreation() bool {
	return t.To == nil
}

// IsContractInteraction 检查是否为合约交互
func (t *Transaction) IsContractInteraction() bool {
	return t.To != nil && len(t.Input) > 2
}

// IsLargeTransaction 检查是否为大额交易
func (t *Transaction) IsLargeTransaction() bool {
	return t.GetValueInEther() >= LargeTransactionThreshold
}

// GetGasPriceInGwei 获取 Gas 价格（Gwei）
func (t *Transaction) GetGasPriceInGwei() float64 {
	var gasPriceWei *big.Int
	
	switch t.Type {
	case TxTypeLegacy, TxTypeAccessList:
		if t.GasPrice == nil {
			return 0
		}
		var ok bool
		gasPriceWei, ok = new(big.Int).SetString(*t.GasPrice, 10)
		if !ok {
			return 0
		}
	case TxTypeDynamicFee:
		if t.EffectiveGasPrice == nil {
			return 0
		}
		var ok bool
		gasPriceWei, ok = new(big.Int).SetString(*t.EffectiveGasPrice, 10)
		if !ok {
			return 0
		}
	default:
		return 0
	}
	
	gasPriceGwei := new(big.Float).SetInt(gasPriceWei)
	gasPriceGwei.Quo(gasPriceGwei, big.NewFloat(WeiPerGwei))
	
	result, _ := gasPriceGwei.Float64()
	return result
}

// GetGasEfficiency 计算 Gas 效率（实际使用/限制）
func (t *Transaction) GetGasEfficiency() float64 {
	if t.GasUsed == nil || t.Gas == 0 {
		return 0
	}
	return float64(*t.GasUsed) / float64(t.Gas) * 100
}

// ToJSON 序列化为 JSON
func (t *Transaction) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON 从 JSON 反序列化
func (t *Transaction) FromJSON(data []byte) error {
	return json.Unmarshal(data, t)
}

// ToCompactJSON 序列化为紧凑 JSON（排除关联数据）
func (t *Transaction) ToCompactJSON() ([]byte, error) {
	compactTx := struct {
		ID              uint64            `json:"id"`
		Hash            string            `json:"hash"`
		BlockNumber     uint64            `json:"block_number"`
		From            string            `json:"from_address"`
		To              *string           `json:"to_address"`
		Value           string            `json:"value"`
		ValueInEther    float64           `json:"value_in_ether"`
		Gas             uint64            `json:"gas"`
		GasUsed         *uint64           `json:"gas_used"`
		GasCostInEther  float64           `json:"gas_cost_in_ether"`
		Type            TransactionType   `json:"type"`
		Status          TransactionStatus `json:"status"`
		Timestamp       time.Time         `json:"timestamp"`
		IsContractCall  bool              `json:"is_contract_call"`
		IsLargeValue    bool              `json:"is_large_value"`
		CreatedAt       time.Time         `json:"created_at"`
	}{
		ID:              t.ID,
		Hash:            t.Hash,
		BlockNumber:     t.BlockNumber,
		From:            t.From,
		To:              t.To,
		Value:           t.Value,
		ValueInEther:    t.GetValueInEther(),
		Gas:             t.Gas,
		GasUsed:         t.GasUsed,
		GasCostInEther:  t.GetGasCostInEther(),
		Type:            t.Type,
		Status:          t.Status,
		Timestamp:       t.Timestamp,
		IsContractCall:  t.IsContractInteraction(),
		IsLargeValue:    t.IsLargeTransaction(),
		CreatedAt:       t.CreatedAt,
	}
	return json.Marshal(compactTx)
}

// TransactionLog 相关方法

// Validate 验证交易日志数据
func (tl *TransactionLog) Validate() error {
	validate := validator.New()
	if err := validate.Struct(tl); err != nil {
		return err
	}
	
	// 验证交易哈希格式
	if len(tl.TransactionHash) != 66 || tl.TransactionHash[:2] != "0x" {
		return errors.New("invalid transaction hash format")
	}
	
	// 验证地址格式
	if len(tl.Address) != 42 || tl.Address[:2] != "0x" {
		return errors.New("invalid address format")
	}
	
	return nil
}

// GetTopics 获取解析后的主题数组
func (tl *TransactionLog) GetTopics() ([]string, error) {
	if tl.Topics == "" {
		return []string{}, nil
	}
	
	var topics []string
	err := json.Unmarshal([]byte(tl.Topics), &topics)
	return topics, err
}

// SetTopics 设置主题数组
func (tl *TransactionLog) SetTopics(topics []string) error {
	data, err := json.Marshal(topics)
	if err != nil {
		return err
	}
	tl.Topics = string(data)
	return nil
}

// ToJSON 序列化为 JSON
func (tl *TransactionLog) ToJSON() ([]byte, error) {
	return json.Marshal(tl)
}

// FromJSON 从 JSON 反序列化
func (tl *TransactionLog) FromJSON(data []byte) error {
	return json.Unmarshal(data, tl)
}

// 交易查询参数
type TransactionQueryParams struct {
	PaginationParams
	FilterParams
	
	// 交易特定过滤条件
	BlockNumber     *uint64           `json:"block_number"`
	MinBlockNumber  *uint64           `json:"min_block_number"`
	MaxBlockNumber  *uint64           `json:"max_block_number"`
	FromAddress     string            `json:"from_address"`
	ToAddress       string            `json:"to_address"`
	MinValue        *string           `json:"min_value"` // Wei 单位
	MaxValue        *string           `json:"max_value"` // Wei 单位
	MinGasUsed      *uint64           `json:"min_gas_used"`
	MaxGasUsed      *uint64           `json:"max_gas_used"`
	TxType          TransactionType   `json:"tx_type"`
	TxStatus        TransactionStatus `json:"tx_status"`
	ContractOnly    *bool             `json:"contract_only"`
	LargeValueOnly  *bool             `json:"large_value_only"`
}

// BuildWhereClause 构建查询条件
func (q *TransactionQueryParams) BuildWhereClause() (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1
	
	// 区块号过滤
	if q.BlockNumber != nil {
		conditions = append(conditions, "block_number = $"+string(rune(argIndex+'0')))
		args = append(args, *q.BlockNumber)
		argIndex++
	}
	if q.MinBlockNumber != nil {
		conditions = append(conditions, "block_number >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MinBlockNumber)
		argIndex++
	}
	if q.MaxBlockNumber != nil {
		conditions = append(conditions, "block_number <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MaxBlockNumber)
		argIndex++
	}
	
	// 地址过滤
	if q.FromAddress != "" {
		conditions = append(conditions, "from_address = $"+string(rune(argIndex+'0')))
		args = append(args, q.FromAddress)
		argIndex++
	}
	if q.ToAddress != "" {
		conditions = append(conditions, "to_address = $"+string(rune(argIndex+'0')))
		args = append(args, q.ToAddress)
		argIndex++
	}
	
	// 交易类型过滤
	if q.TxType != "" {
		conditions = append(conditions, "type = $"+string(rune(argIndex+'0')))
		args = append(args, q.TxType)
		argIndex++
	}
	
	// 交易状态过滤
	if q.TxStatus != "" {
		conditions = append(conditions, "status = $"+string(rune(argIndex+'0')))
		args = append(args, q.TxStatus)
		argIndex++
	}
	
	// 合约交易过滤
	if q.ContractOnly != nil && *q.ContractOnly {
		conditions = append(conditions, "to_address IS NOT NULL AND LENGTH(input) > 2")
	}
	
	// 时间范围
	if q.StartTime != nil {
		conditions = append(conditions, "timestamp >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.StartTime)
		argIndex++
	}
	if q.EndTime != nil {
		conditions = append(conditions, "timestamp <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.EndTime)
		argIndex++
	}
	
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}
	
	return whereClause, args
}

// 交易创建请求结构
type CreateTransactionRequest struct {
	Hash                 string            `json:"hash" validate:"required,len=66"`
	BlockNumber          uint64            `json:"block_number" validate:"required,min=0"`
	BlockHash            string            `json:"block_hash" validate:"len=66"`
	Index                uint32            `json:"transaction_index"`
	From                 string            `json:"from_address" validate:"required,len=42"`
	To                   *string           `json:"to_address" validate:"omitempty,len=42"`
	Value                string            `json:"value" validate:"required"`
	Input                string            `json:"input"`
	Gas                  uint64            `json:"gas" validate:"required,min=0"`
	GasUsed              *uint64           `json:"gas_used" validate:"omitempty,min=0"`
	GasPrice             *string           `json:"gas_price"`
	MaxFeePerGas         *string           `json:"max_fee_per_gas"`
	MaxPriorityFeePerGas *string           `json:"max_priority_fee_per_gas"`
	Type                 TransactionType   `json:"type" validate:"required"`
	Status               TransactionStatus `json:"status" validate:"required"`
	Nonce                uint64            `json:"nonce"`
	V                    string            `json:"v"`
	R                    string            `json:"r" validate:"len=66"`
	S                    string            `json:"s" validate:"len=66"`
	CumulativeGasUsed    *uint64           `json:"cumulative_gas_used"`
	EffectiveGasPrice    *string           `json:"effective_gas_price"`
	ContractAddress      *string           `json:"contract_address" validate:"omitempty,len=42"`
	LogsCount            uint32            `json:"logs_count"`
	LogsBloom            string            `json:"logs_bloom"`
	Timestamp            int64             `json:"timestamp" validate:"required"`
}

// ToTransaction 转换为交易模型
func (r *CreateTransactionRequest) ToTransaction() *Transaction {
	return &Transaction{
		Hash:                 r.Hash,
		BlockNumber:          r.BlockNumber,
		BlockHash:            r.BlockHash,
		Index:                r.Index,
		From:                 r.From,
		To:                   r.To,
		Value:                r.Value,
		Input:                r.Input,
		Gas:                  r.Gas,
		GasUsed:              r.GasUsed,
		GasPrice:             r.GasPrice,
		MaxFeePerGas:         r.MaxFeePerGas,
		MaxPriorityFeePerGas: r.MaxPriorityFeePerGas,
		Type:                 r.Type,
		Status:               r.Status,
		Nonce:                r.Nonce,
		V:                    r.V,
		R:                    r.R,
		S:                    r.S,
		CumulativeGasUsed:    r.CumulativeGasUsed,
		EffectiveGasPrice:    r.EffectiveGasPrice,
		ContractAddress:      r.ContractAddress,
		LogsCount:            r.LogsCount,
		LogsBloom:            r.LogsBloom,
		Timestamp:            time.Unix(r.Timestamp, 0),
	}
}

// Validate 验证创建请求
func (r *CreateTransactionRequest) Validate() error {
	validate := validator.New()
	return validate.Struct(r)
}
