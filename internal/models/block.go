package models

import (
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/go-playground/validator/v10"
)

// Block 区块数据模型
type Block struct {
	BaseModel

	// 区块基本信息
	//区块高度
	Number uint64 `json:"number" gorm:"uniqueIndex;not null" validate:"required,min=0"`
	//区块哈希
	Hash string `json:"hash" gorm:"uniqueIndex;size:66;not null" validate:"required,len=66"`
	//父区块哈希
	ParentHash string `json:"parent_hash" gorm:"size:66;not null" validate:"required,len=66"`
	//区块时间戳
	Timestamp time.Time `json:"timestamp" gorm:"index;not null" validate:"required"`

	// 区块头信息
	//矿工地址
	Miner string `json:"miner" gorm:"size:42;index" validate:"required,len=42"`
	//难度
	Difficulty string `json:"difficulty" gorm:"type:varchar(78)" validate:"required"`
	//总难度
	TotalDifficulty string `json:"total_difficulty" gorm:"type:varchar(78)"`
	//区块大小
	Size uint64 `json:"size" validate:"min=0"`
	//区块 gas 限制
	GasLimit uint64 `json:"gas_limit" validate:"required,min=0"`
	//区块 gas 使用量
	GasUsed uint64 `json:"gas_used" validate:"required,min=0"`

	// 交易统计
	//交易数量
	TransactionCount uint32 `json:"transaction_count" validate:"min=0"`

	// 状态根和收据根
	//状态根
	StateRoot string `json:"state_root" gorm:"size:66" validate:"len=66"`
	//收据根
	ReceiptsRoot string `json:"receipts_root" gorm:"size:66" validate:"len=66"`
	//交易根
	TransactionsRoot string `json:"transactions_root" gorm:"size:66" validate:"len=66"`

	// 额外数据
	//额外数据
	ExtraData string `json:"extra_data" gorm:"type:text"`
	//混合哈希
	MixHash string `json:"mix_hash" gorm:"size:66" validate:"len=66"`
	//nonce
	Nonce string `json:"nonce" gorm:"size:18"`

	// Bloom 过滤器
	//日志 Bloom 过滤器
	LogsBloom string `json:"logs_bloom" gorm:"type:text"`

	// 基础费用 (EIP-1559)
	//基础费用
	BaseFeePerGas *string `json:"base_fee_per_gas" gorm:"type:varchar(78)"`

	// 关联关系
	Transactions []Transaction `json:"transactions,omitempty"`

	// 计算字段（不存储到数据库）
	//Gas 利用率
	GasUtilization float64 `json:"gas_utilization" gorm:"-"`
	//区块时间
	BlockTime float64 `json:"block_time" gorm:"-"` // 与上一个区块的时间间隔
	//总交易费用
	TransactionFees string `json:"transaction_fees" gorm:"-"` // 总交易费用
	//矿工奖励
	MinerReward string `json:"miner_reward" gorm:"-"` // 矿工奖励
	//叔块奖励
	UncleReward string `json:"uncle_reward" gorm:"-"` // 叔块奖励
	//是否为空区块
	IsEmpty bool `json:"is_empty" gorm:"-"` // 是否为空区块
}

// BlockStatistics 区块统计信息
type BlockStatistics struct {
	//区块高度
	BlockNumber uint64 `json:"block_number"`
	//区块时间戳
	Timestamp time.Time `json:"timestamp"`
	//Gas 利用率
	GasUtilization float64 `json:"gas_utilization"`
	//交易数量
	TransactionCount uint32 `json:"transaction_count"`
	//平均 gas 价格
	AverageGasPrice string `json:"average_gas_price"`
	//总交易费用
	TotalTransactionFees string `json:"total_transaction_fees"`
	//区块时间
	BlockTime float64 `json:"block_time"`
	//区块大小
	BlockSize uint64 `json:"block_size"`
}

// TableName 指定表名
func (Block) TableName() string {
	return "blocks"
}

// BeforeSave 保存前的钩子函数
func (b *Block) BeforeSave() error {
	// 计算 Gas 利用率
	if b.GasLimit > 0 {
		b.GasUtilization = float64(b.GasUsed) / float64(b.GasLimit) * 100
	}

	// 检查是否为空区块
	b.IsEmpty = b.TransactionCount == 0

	return nil
}

// Validate 验证区块数据
func (b *Block) Validate() error {
	validate := validator.New()
	if err := validate.Struct(b); err != nil {
		return err
	}

	// 验证哈希格式
	if len(b.Hash) != 66 || b.Hash[:2] != "0x" {
		return errors.New("invalid block hash format")
	}

	if len(b.ParentHash) != 66 || b.ParentHash[:2] != "0x" {
		return errors.New("invalid parent hash format")
	}

	// 验证矿工地址格式
	if len(b.Miner) != 42 || b.Miner[:2] != "0x" {
		return errors.New("invalid miner address format")
	}

	// 验证 Gas 使用量不能超过限制
	if b.GasUsed > b.GasLimit {
		return errors.New("gas used cannot exceed gas limit")
	}

	return nil
}

// GetGasUtilizationPercentage 获取 Gas 利用率百分比
func (b *Block) GetGasUtilizationPercentage() float64 {
	if b.GasLimit == 0 {
		return 0
	}
	return float64(b.GasUsed) / float64(b.GasLimit) * 100
}

// IsEmptyBlock 检查是否为空区块
func (b *Block) IsEmptyBlock() bool {
	return b.TransactionCount == 0
}

// GetDifficulty 获取难度值（big.Int）
func (b *Block) GetDifficulty() (*big.Int, error) {
	difficulty := new(big.Int)
	if b.Difficulty == "" {
		return difficulty, nil
	}

	// 去掉 0x 前缀
	diffStr := b.Difficulty
	if len(diffStr) > 2 && diffStr[:2] == "0x" {
		diffStr = diffStr[2:]
	}

	difficulty, ok := difficulty.SetString(diffStr, 16)
	if !ok {
		return nil, errors.New("invalid difficulty format")
	}

	return difficulty, nil
}

// GetTotalDifficulty 获取总难度值（big.Int）
func (b *Block) GetTotalDifficulty() (*big.Int, error) {
	totalDifficulty := new(big.Int)
	if b.TotalDifficulty == "" {
		return totalDifficulty, nil
	}

	// 去掉 0x 前缀
	diffStr := b.TotalDifficulty
	if len(diffStr) > 2 && diffStr[:2] == "0x" {
		diffStr = diffStr[2:]
	}

	totalDifficulty, ok := totalDifficulty.SetString(diffStr, 16)
	if !ok {
		return nil, errors.New("invalid total difficulty format")
	}

	return totalDifficulty, nil
}

// GetBaseFeePerGas 获取基础费用（big.Int）
func (b *Block) GetBaseFeePerGas() (*big.Int, error) {
	if b.BaseFeePerGas == nil {
		return nil, nil
	}

	baseFee := new(big.Int)
	baseFee, ok := baseFee.SetString(*b.BaseFeePerGas, 10)
	if !ok {
		return nil, errors.New("invalid base fee format")
	}

	return baseFee, nil
}

// CalculateBlockTime 计算与上一个区块的时间间隔
func (b *Block) CalculateBlockTime(prevBlockTimestamp time.Time) float64 {
	if prevBlockTimestamp.IsZero() {
		return 0
	}
	return b.Timestamp.Sub(prevBlockTimestamp).Seconds()
}

// IsSlowBlock 检查是否为慢区块
func (b *Block) IsSlowBlock(prevBlockTimestamp time.Time) bool {
	blockTime := b.CalculateBlockTime(prevBlockTimestamp)
	return blockTime > SlowBlockTime
}

// IsCongested 检查区块是否拥堵
func (b *Block) IsCongested() bool {
	return b.GetGasUtilizationPercentage() > CongestionThreshold
}

// GetMinerReward 计算矿工奖励（简化版本）
func (b *Block) GetMinerReward() *big.Int {
	// 以太坊 2.0 之前的区块奖励
	// 这里简化处理，实际需要根据区块高度和网络升级来计算
	baseReward := big.NewInt(2e18) // 2 ETH (wei)

	// 根据区块号调整奖励
	if b.Number >= 4370000 { // Byzantium
		baseReward = big.NewInt(3e18) // 3 ETH
	}
	if b.Number >= 7280000 { // Constantinople
		baseReward = big.NewInt(2e18) // 2 ETH
	}

	return baseReward
}

// ToStatistics 转换为统计信息
func (b *Block) ToStatistics() *BlockStatistics {
	return &BlockStatistics{
		BlockNumber:      b.Number,
		Timestamp:        b.Timestamp,
		GasUtilization:   b.GetGasUtilizationPercentage(),
		TransactionCount: b.TransactionCount,
		BlockTime:        b.BlockTime,
		BlockSize:        b.Size,
	}
}

// ToJSON 序列化为 JSON
func (b *Block) ToJSON() ([]byte, error) {
	return json.Marshal(b)
}

// FromJSON 从 JSON 反序列化
func (b *Block) FromJSON(data []byte) error {
	return json.Unmarshal(data, b)
}

// ToCompactJSON 序列化为紧凑 JSON（排除关联数据）
func (b *Block) ToCompactJSON() ([]byte, error) {
	compactBlock := struct {
		ID               uint64    `json:"id"`
		Number           uint64    `json:"number"`
		Hash             string    `json:"hash"`
		Timestamp        time.Time `json:"timestamp"`
		Miner            string    `json:"miner"`
		GasLimit         uint64    `json:"gas_limit"`
		GasUsed          uint64    `json:"gas_used"`
		TransactionCount uint32    `json:"transaction_count"`
		GasUtilization   float64   `json:"gas_utilization"`
		Size             uint64    `json:"size"`
		CreatedAt        time.Time `json:"created_at"`
	}{
		ID:               b.ID,
		Number:           b.Number,
		Hash:             b.Hash,
		Timestamp:        b.Timestamp,
		Miner:            b.Miner,
		GasLimit:         b.GasLimit,
		GasUsed:          b.GasUsed,
		TransactionCount: b.TransactionCount,
		GasUtilization:   b.GetGasUtilizationPercentage(),
		Size:             b.Size,
		CreatedAt:        b.CreatedAt,
	}
	return json.Marshal(compactBlock)
}

// 区块查询参数
type BlockQueryParams struct {
	PaginationParams
	FilterParams

	// 区块特定过滤条件
	MinNumber   *uint64 `json:"min_number"`
	MaxNumber   *uint64 `json:"max_number"`
	Miner       string  `json:"miner"`
	MinGasUsed  *uint64 `json:"min_gas_used"`
	MaxGasUsed  *uint64 `json:"max_gas_used"`
	MinTxCount  *uint32 `json:"min_tx_count"`
	MaxTxCount  *uint32 `json:"max_tx_count"`
	EmptyBlocks *bool   `json:"empty_blocks"`
}

// BuildWhereClause 构建查询条件
func (q *BlockQueryParams) BuildWhereClause() (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// 区块号范围
	if q.MinNumber != nil {
		conditions = append(conditions, "number >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MinNumber)
		argIndex++
	}
	if q.MaxNumber != nil {
		conditions = append(conditions, "number <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MaxNumber)
		argIndex++
	}

	// 矿工过滤
	if q.Miner != "" {
		conditions = append(conditions, "miner = $"+string(rune(argIndex+'0')))
		args = append(args, q.Miner)
		argIndex++
	}

	// Gas 使用量范围
	if q.MinGasUsed != nil {
		conditions = append(conditions, "gas_used >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MinGasUsed)
		argIndex++
	}
	if q.MaxGasUsed != nil {
		conditions = append(conditions, "gas_used <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MaxGasUsed)
		argIndex++
	}

	// 交易数量范围
	if q.MinTxCount != nil {
		conditions = append(conditions, "transaction_count >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MinTxCount)
		argIndex++
	}
	if q.MaxTxCount != nil {
		conditions = append(conditions, "transaction_count <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.MaxTxCount)
		argIndex++
	}

	// 空区块过滤
	if q.EmptyBlocks != nil {
		if *q.EmptyBlocks {
			conditions = append(conditions, "transaction_count = 0")
		} else {
			conditions = append(conditions, "transaction_count > 0")
		}
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

// 区块创建请求结构
type CreateBlockRequest struct {
	Number           uint64  `json:"number" validate:"required,min=0"`
	Hash             string  `json:"hash" validate:"required,len=66"`
	ParentHash       string  `json:"parent_hash" validate:"required,len=66"`
	Timestamp        int64   `json:"timestamp" validate:"required"`
	Miner            string  `json:"miner" validate:"required,len=42"`
	Difficulty       string  `json:"difficulty" validate:"required"`
	TotalDifficulty  string  `json:"total_difficulty"`
	Size             uint64  `json:"size"`
	GasLimit         uint64  `json:"gas_limit" validate:"required,min=0"`
	GasUsed          uint64  `json:"gas_used" validate:"required,min=0"`
	TransactionCount uint32  `json:"transaction_count"`
	StateRoot        string  `json:"state_root" validate:"len=66"`
	ReceiptsRoot     string  `json:"receipts_root" validate:"len=66"`
	TransactionsRoot string  `json:"transactions_root" validate:"len=66"`
	ExtraData        string  `json:"extra_data"`
	MixHash          string  `json:"mix_hash" validate:"len=66"`
	Nonce            string  `json:"nonce"`
	LogsBloom        string  `json:"logs_bloom"`
	BaseFeePerGas    *string `json:"base_fee_per_gas"`
}

// ToBlock 转换为区块模型
func (r *CreateBlockRequest) ToBlock() *Block {
	return &Block{
		Number:           r.Number,
		Hash:             r.Hash,
		ParentHash:       r.ParentHash,
		Timestamp:        time.Unix(r.Timestamp, 0),
		Miner:            r.Miner,
		Difficulty:       r.Difficulty,
		TotalDifficulty:  r.TotalDifficulty,
		Size:             r.Size,
		GasLimit:         r.GasLimit,
		GasUsed:          r.GasUsed,
		TransactionCount: r.TransactionCount,
		StateRoot:        r.StateRoot,
		ReceiptsRoot:     r.ReceiptsRoot,
		TransactionsRoot: r.TransactionsRoot,
		ExtraData:        r.ExtraData,
		MixHash:          r.MixHash,
		Nonce:            r.Nonce,
		LogsBloom:        r.LogsBloom,
		BaseFeePerGas:    r.BaseFeePerGas,
	}
}

// Validate 验证创建请求
func (r *CreateBlockRequest) Validate() error {
	validate := validator.New()
	return validate.Struct(r)
}
