package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/go-playground/validator/v10"
)

// User 用户模型
type User struct {
	BaseModel
	
	// 用户基本信息
	Username    string     `json:"username" gorm:"uniqueIndex;size:50;not null" validate:"required,min=3,max=50,alphanum"`
	Email       string     `json:"email" gorm:"uniqueIndex;size:255;not null" validate:"required,email"`
	Password    string     `json:"-" gorm:"size:255;not null" validate:"required,min=8"`
	FullName    string     `json:"full_name" gorm:"size:100" validate:"max=100"`
	Avatar      string     `json:"avatar" gorm:"size:500"`
	
	// 用户状态和角色
	Role        UserRole   `json:"role" gorm:"type:varchar(20);index;not null;default:'user'" validate:"required"`
	Status      UserStatus `json:"status" gorm:"type:varchar(20);index;not null;default:'active'" validate:"required"`
	
	// 联系方式
	Phone       string     `json:"phone" gorm:"size:20" validate:"omitempty,e164"`
	TelegramID  string     `json:"telegram_id" gorm:"size:50;index"`
	
	// 偏好设置 (JSON 格式)
	Preferences string     `json:"preferences" gorm:"type:text"`
	Timezone    string     `json:"timezone" gorm:"size:50;default:'UTC'" validate:"required"`
	Language    string     `json:"language" gorm:"size:10;default:'en'" validate:"required"`
	
	// 安全相关
	LastLoginAt      *time.Time `json:"last_login_at"`
	LastLoginIP      string     `json:"last_login_ip" gorm:"size:45"`
	FailedLoginCount int32      `json:"failed_login_count" validate:"min=0"`
	LockedUntil      *time.Time `json:"locked_until"`
	
	// 邮箱验证
	EmailVerified   bool       `json:"email_verified" gorm:"default:false"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	
	// API 访问
	APIKey          string     `json:"api_key" gorm:"uniqueIndex;size:64"`
	APIKeyCreatedAt *time.Time `json:"api_key_created_at"`
	
	// 关联关系
	AlertRules    []AlertRule    `json:"alert_rules,omitempty"`
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
	Sessions      []UserSession  `json:"sessions,omitempty"`
}

// UserPreferences 用户偏好设置结构
type UserPreferences struct {
	Theme                 string `json:"theme" validate:"oneof=light dark auto"`
	NotificationSound     bool   `json:"notification_sound"`
	EmailNotifications    bool   `json:"email_notifications"`
	TelegramNotifications bool   `json:"telegram_notifications"`
	DashboardRefreshRate  int32  `json:"dashboard_refresh_rate" validate:"min=5,max=300"` // 秒
	DefaultTimeRange      string `json:"default_time_range" validate:"oneof=1h 6h 24h 7d 30d"`
	DateFormat            string `json:"date_format" validate:"oneof=YYYY-MM-DD DD/MM/YYYY MM/DD/YYYY"`
	Currency              string `json:"currency" validate:"oneof=USD EUR CNY"`
	Language              string `json:"language" validate:"oneof=en zh-CN zh-TW"`
}

// UserSession 用户会话模型
type UserSession struct {
	BaseModel
	
	UserID    uint64    `json:"user_id" gorm:"index;not null"`
	SessionID string    `json:"session_id" gorm:"uniqueIndex;size:128;not null"`
	IPAddress string    `json:"ip_address" gorm:"size:45"`
	UserAgent string    `json:"user_agent" gorm:"type:text"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	
	// 关联关系
	User User `json:"user,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

func (UserSession) TableName() string {
	return "user_sessions"
}

// BeforeSave 保存前的钩子函数
func (u *User) BeforeSave() error {
	// 如果密码已修改，进行哈希处理
	if u.Password != "" && len(u.Password) < 60 { // bcrypt 哈希长度通常为 60
		if err := u.HashPassword(); err != nil {
			return err
		}
	}
	
	// 生成 API Key（如果没有）
	if u.APIKey == "" {
		if err := u.GenerateAPIKey(); err != nil {
			return err
		}
	}
	
	return nil
}

// Validate 验证用户数据
func (u *User) Validate() error {
	validate := validator.New()
	if err := validate.Struct(u); err != nil {
		return err
	}
	
	// 验证用户角色
	if !u.Role.IsValid() {
		return errors.New("invalid role")
	}
	
	// 验证用户状态
	if !u.Status.IsValid() {
		return errors.New("invalid status")
	}
	
	return nil
}

// HashPassword 密码哈希
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword 验证密码
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// GenerateAPIKey 生成 API Key
func (u *User) GenerateAPIKey() error {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	u.APIKey = hex.EncodeToString(bytes)
	now := time.Now()
	u.APIKeyCreatedAt = &now
	return nil
}

// RegenerateAPIKey 重新生成 API Key
func (u *User) RegenerateAPIKey() error {
	return u.GenerateAPIKey()
}

// GetPreferences 获取用户偏好设置
func (u *User) GetPreferences() (*UserPreferences, error) {
	if u.Preferences == "" {
		// 返回默认偏好设置
		return &UserPreferences{
			Theme:                 "light",
			NotificationSound:     true,
			EmailNotifications:    true,
			TelegramNotifications: true,
			DashboardRefreshRate:  30,
			DefaultTimeRange:      "24h",
			DateFormat:            "YYYY-MM-DD",
			Currency:              "USD",
			Language:              "en",
		}, nil
	}
	
	var prefs UserPreferences
	err := json.Unmarshal([]byte(u.Preferences), &prefs)
	return &prefs, err
}

// SetPreferences 设置用户偏好
func (u *User) SetPreferences(prefs *UserPreferences) error {
	// 验证偏好设置
	validate := validator.New()
	if err := validate.Struct(prefs); err != nil {
		return err
	}
	
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	u.Preferences = string(data)
	return nil
}

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// CanLogin 检查用户是否可以登录
func (u *User) CanLogin() bool {
	if !u.IsActive() {
		return false
	}
	
	// 检查是否被锁定
	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return false
	}
	
	return true
}

// IsLocked 检查用户是否被锁定
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && time.Now().Before(*u.LockedUntil)
}

// LockAccount 锁定账户
func (u *User) LockAccount(duration time.Duration) {
	lockUntil := time.Now().Add(duration)
	u.LockedUntil = &lockUntil
}

// UnlockAccount 解锁账户
func (u *User) UnlockAccount() {
	u.LockedUntil = nil
	u.FailedLoginCount = 0
}

// IncrementFailedLogin 增加失败登录次数
func (u *User) IncrementFailedLogin() {
	u.FailedLoginCount++
	
	// 如果失败次数达到阈值，锁定账户
	if u.FailedLoginCount >= 5 {
		u.LockAccount(30 * time.Minute) // 锁定 30 分钟
	}
}

// ResetFailedLogin 重置失败登录次数
func (u *User) ResetFailedLogin() {
	u.FailedLoginCount = 0
}

// UpdateLastLogin 更新最后登录信息
func (u *User) UpdateLastLogin(ip string) {
	now := time.Now()
	u.LastLoginAt = &now
	u.LastLoginIP = ip
	u.ResetFailedLogin()
}

// VerifyEmail 验证邮箱
func (u *User) VerifyEmail() {
	u.EmailVerified = true
	now := time.Now()
	u.EmailVerifiedAt = &now
}

// HasPermission 检查用户是否有指定权限
func (u *User) HasPermission(permission string) bool {
	if !u.IsActive() {
		return false
	}
	return u.Role.HasPermission(permission)
}

// IsAdmin 检查是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// ToJSON 序列化为 JSON（排除敏感信息）
func (u *User) ToJSON() ([]byte, error) {
	// 创建一个副本，排除密码等敏感信息
	userCopy := *u
	userCopy.Password = ""
	userCopy.APIKey = ""
	return json.Marshal(userCopy)
}

// ToPublicJSON 序列化为公开 JSON（更少信息）
func (u *User) ToPublicJSON() ([]byte, error) {
	publicUser := struct {
		ID        uint64    `json:"id"`
		Username  string    `json:"username"`
		FullName  string    `json:"full_name"`
		Avatar    string    `json:"avatar"`
		Role      UserRole  `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}{
		ID:        u.ID,
		Username:  u.Username,
		FullName:  u.FullName,
		Avatar:    u.Avatar,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
	return json.Marshal(publicUser)
}

// GetDisplayName 获取显示名称
func (u *User) GetDisplayName() string {
	if u.FullName != "" {
		return u.FullName
	}
	return u.Username
}

// GetAvatarURL 获取头像 URL
func (u *User) GetAvatarURL() string {
	if u.Avatar != "" {
		return u.Avatar
	}
	// 返回默认头像或 Gravatar
	return "https://www.gravatar.com/avatar/" + u.Email + "?d=identicon"
}

// UserSession 相关方法

// IsExpired 检查会话是否过期
func (s *UserSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid 检查会话是否有效
func (s *UserSession) IsValid() bool {
	return s.IsActive && !s.IsExpired()
}

// Extend 延长会话时间
func (s *UserSession) Extend(duration time.Duration) {
	s.ExpiresAt = time.Now().Add(duration)
}

// Invalidate 使会话失效
func (s *UserSession) Invalidate() {
	s.IsActive = false
}

// Validate 验证会话数据
func (s *UserSession) Validate() error {
	validate := validator.New()
	return validate.Struct(s)
}

// 用户创建请求结构
type CreateUserRequest struct {
	Username string   `json:"username" validate:"required,min=3,max=50,alphanum"`
	Email    string   `json:"email" validate:"required,email"`
	Password string   `json:"password" validate:"required,min=8"`
	FullName string   `json:"full_name" validate:"max=100"`
	Role     UserRole `json:"role" validate:"omitempty"`
	Phone    string   `json:"phone" validate:"omitempty,e164"`
}

// ToUser 转换为用户模型
func (r *CreateUserRequest) ToUser() *User {
	user := &User{
		Username: r.Username,
		Email:    r.Email,
		Password: r.Password,
		FullName: r.FullName,
		Role:     r.Role,
		Phone:    r.Phone,
		Status:   UserStatusActive,
		Timezone: "UTC",
		Language: "en",
	}
	
	// 如果没有指定角色，默认为普通用户
	if user.Role == "" {
		user.Role = RoleUser
	}
	
	return user
}

// 用户更新请求结构
type UpdateUserRequest struct {
	FullName    *string   `json:"full_name" validate:"omitempty,max=100"`
	Avatar      *string   `json:"avatar" validate:"omitempty,max=500"`
	Phone       *string   `json:"phone" validate:"omitempty,e164"`
	TelegramID  *string   `json:"telegram_id" validate:"omitempty,max=50"`
	Timezone    *string   `json:"timezone" validate:"omitempty,max=50"`
	Language    *string   `json:"language" validate:"omitempty,max=10"`
	Preferences *string   `json:"preferences"`
}

// ApplyToUser 应用更新到用户模型
func (r *UpdateUserRequest) ApplyToUser(user *User) {
	if r.FullName != nil {
		user.FullName = *r.FullName
	}
	if r.Avatar != nil {
		user.Avatar = *r.Avatar
	}
	if r.Phone != nil {
		user.Phone = *r.Phone
	}
	if r.TelegramID != nil {
		user.TelegramID = *r.TelegramID
	}
	if r.Timezone != nil {
		user.Timezone = *r.Timezone
	}
	if r.Language != nil {
		user.Language = *r.Language
	}
	if r.Preferences != nil {
		user.Preferences = *r.Preferences
	}
}

// 密码修改请求结构
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// Validate 验证密码修改请求
func (r *ChangePasswordRequest) Validate() error {
	validate := validator.New()
	return validate.Struct(r)
}
