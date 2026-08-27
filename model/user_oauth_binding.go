package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrUserOAuthBindingExists = errors.New("user already has another OAuth binding")

// UserOAuthBinding stores the binding relationship between users and custom OAuth providers
type UserOAuthBinding struct {
	Id             int       `json:"id" gorm:"primaryKey"`
	UserId         int       `json:"user_id" gorm:"not null;uniqueIndex:ux_user_provider"`                                    // User ID - one binding per user per provider
	ProviderId     int       `json:"provider_id" gorm:"not null;uniqueIndex:ux_user_provider;uniqueIndex:ux_provider_userid"` // Custom OAuth provider ID
	ProviderUserId string    `json:"provider_user_id" gorm:"type:varchar(256);not null;uniqueIndex:ux_provider_userid"`       // User ID from OAuth provider - one OAuth account per provider
	CreatedAt      time.Time `json:"created_at"`
}

func (UserOAuthBinding) TableName() string {
	return "user_oauth_bindings"
}

// GetUserOAuthBindingsByUserId returns all OAuth bindings for a user
func GetUserOAuthBindingsByUserId(userId int) ([]*UserOAuthBinding, error) {
	var bindings []*UserOAuthBinding
	err := DB.Where("user_id = ?", userId).Find(&bindings).Error
	return bindings, err
}

// GetUserOAuthBinding returns a specific binding for a user and provider
func GetUserOAuthBinding(userId, providerId int) (*UserOAuthBinding, error) {
	var binding UserOAuthBinding
	err := DB.Where("user_id = ? AND provider_id = ?", userId, providerId).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetUserByOAuthBinding finds a user by provider ID and provider user ID
func GetUserByOAuthBinding(providerId int, providerUserId string) (*User, error) {
	var binding UserOAuthBinding
	err := DB.Where("provider_id = ? AND provider_user_id = ?", providerId, providerUserId).First(&binding).Error
	if err != nil {
		return nil, err
	}

	var user User
	err = DB.First(&user, binding.UserId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsProviderUserIdTaken checks if a provider user ID is already bound to any user
func IsProviderUserIdTaken(providerId int, providerUserId string) bool {
	var count int64
	DB.Model(&UserOAuthBinding{}).Where("provider_id = ? AND provider_user_id = ?", providerId, providerUserId).Count(&count)
	return count > 0
}

// EnsureUserOAuthBindingAvailableWithTx keeps one local account bound to at
// most one third-party login channel. The user row lock serializes competing
// built-in, custom, and Telegram binding attempts on MySQL and PostgreSQL;
// SQLite's single-writer behavior prevents both transactions from committing.
func EnsureUserOAuthBindingAvailableWithTx(tx *gorm.DB, userId int, targetColumn string, targetProviderId int) error {
	if tx == nil {
		return errors.New("database transaction is empty")
	}
	if userId <= 0 {
		return errors.New("user ID is required")
	}

	var user User
	if err := lockForUpdate(tx).
		Select("id", "github_id", "discord_id", "oidc_id", "wechat_id", "telegram_id", "linux_do_id").
		Where("id = ?", userId).
		First(&user).Error; err != nil {
		return err
	}

	builtInBindings := []struct {
		column string
		value  string
	}{
		{column: "github_id", value: user.GitHubId},
		{column: "discord_id", value: user.DiscordId},
		{column: "oidc_id", value: user.OidcId},
		{column: "wechat_id", value: user.WeChatId},
		{column: "telegram_id", value: user.TelegramId},
		{column: "linux_do_id", value: user.LinuxDOId},
	}
	targetKnown := targetProviderId > 0
	for _, binding := range builtInBindings {
		if binding.column == targetColumn {
			targetKnown = true
			continue
		}
		if strings.TrimSpace(binding.value) != "" {
			return ErrUserOAuthBindingExists
		}
	}
	if !targetKnown {
		return errors.New("OAuth binding target is invalid")
	}

	customBindings := tx.Model(&UserOAuthBinding{}).Where("user_id = ?", userId)
	if targetProviderId > 0 {
		customBindings = customBindings.Where("provider_id <> ?", targetProviderId)
	}
	var count int64
	if err := customBindings.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrUserOAuthBindingExists
	}
	return nil
}

// CreateUserOAuthBinding creates a new OAuth binding
func CreateUserOAuthBinding(binding *UserOAuthBinding) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return CreateUserOAuthBindingWithTx(tx, binding)
	})
}

// CreateUserOAuthBindingWithTx creates a new OAuth binding within a transaction
func CreateUserOAuthBindingWithTx(tx *gorm.DB, binding *UserOAuthBinding) error {
	if tx == nil {
		return errors.New("database transaction is empty")
	}
	if binding == nil {
		return errors.New("OAuth binding is required")
	}
	if binding.UserId == 0 {
		return errors.New("user ID is required")
	}
	if binding.ProviderId == 0 {
		return errors.New("provider ID is required")
	}
	if binding.ProviderUserId == "" {
		return errors.New("provider user ID is required")
	}
	if err := EnsureUserOAuthBindingAvailableWithTx(tx, binding.UserId, "", binding.ProviderId); err != nil {
		return err
	}

	// Check if this provider user ID is already taken (use tx to check within the same transaction)
	var count int64
	tx.Model(&UserOAuthBinding{}).Where("provider_id = ? AND provider_user_id = ?", binding.ProviderId, binding.ProviderUserId).Count(&count)
	if count > 0 {
		return errors.New("this OAuth account is already bound to another user")
	}

	binding.CreatedAt = time.Now()
	return tx.Create(binding).Error
}

// UpdateUserOAuthBinding updates an existing OAuth binding (e.g., rebind to different OAuth account)
func UpdateUserOAuthBinding(userId, providerId int, newProviderUserId string) error {
	if userId <= 0 {
		return errors.New("user ID is required")
	}
	if providerId <= 0 {
		return errors.New("provider ID is required")
	}
	if newProviderUserId == "" {
		return errors.New("provider user ID is required")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := EnsureUserOAuthBindingAvailableWithTx(tx, userId, "", providerId); err != nil {
			return err
		}

		var existingBinding UserOAuthBinding
		err := tx.Where("provider_id = ? AND provider_user_id = ?", providerId, newProviderUserId).
			First(&existingBinding).Error
		if err == nil && existingBinding.UserId != userId {
			return errors.New("this OAuth account is already bound to another user")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var binding UserOAuthBinding
		err = tx.Where("user_id = ? AND provider_id = ?", userId, providerId).First(&binding).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			binding = UserOAuthBinding{
				UserId:         userId,
				ProviderId:     providerId,
				ProviderUserId: newProviderUserId,
				CreatedAt:      time.Now(),
			}
			return tx.Create(&binding).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&binding).Update("provider_user_id", newProviderUserId).Error
	})
}

// DeleteUserOAuthBinding deletes an OAuth binding
func DeleteUserOAuthBinding(userId, providerId int) error {
	return DB.Where("user_id = ? AND provider_id = ?", userId, providerId).Delete(&UserOAuthBinding{}).Error
}

func deleteUserOAuthBindingsByUserId(tx *gorm.DB, userId int) error {
	return tx.Where("user_id = ?", userId).Delete(&UserOAuthBinding{}).Error
}

// GetBindingCountByProviderId returns the number of bindings for a provider
func GetBindingCountByProviderId(providerId int) (int64, error) {
	var count int64
	err := DB.Model(&UserOAuthBinding{}).Where("provider_id = ?", providerId).Count(&count).Error
	return count, err
}
