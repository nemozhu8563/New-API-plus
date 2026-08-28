package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExternalIdentityClaimEnforcesSingleOwnerAtomically(t *testing.T) {
	truncateTables(t)

	first := User{Username: "telegram-owner-one", Password: "password", AffCode: "telegram-owner-one"}
	second := User{Username: "telegram-owner-two", Password: "password", AffCode: "telegram-owner-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, "telegram-123", first.Id)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, "telegram-123", first.Id)
	}))
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, "telegram-123", second.Id)
	})
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, "telegram-456", first.Id)
	})
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	var claims []ExternalIdentityClaim
	require.NoError(t, DB.Find(&claims).Error)
	require.Len(t, claims, 1)
	assert.Equal(t, first.Id, claims[0].UserId)
	assert.Equal(t, "telegram-123", claims[0].Subject)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ReleaseExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, first.Id)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, "telegram-123", second.Id)
	}))
}

func TestExternalIdentityClaimScopesSubjectsByProvider(t *testing.T) {
	truncateTables(t)

	githubUser := User{Username: "github-owner", Password: "password", AffCode: "github-owner"}
	oidcUser := User{Username: "oidc-owner", Password: "password", AffCode: "oidc-owner"}
	require.NoError(t, DB.Create(&githubUser).Error)
	require.NoError(t, DB.Create(&oidcUser).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderGitHub, "shared-subject", githubUser.Id)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderOIDC, "shared-subject", oidcUser.Id)
	}))

	var claims []ExternalIdentityClaim
	require.NoError(t, DB.Order("provider ASC").Find(&claims).Error)
	require.Len(t, claims, 2)
	assert.Equal(t, ExternalIdentityProviderGitHub, claims[0].Provider)
	assert.Equal(t, githubUser.Id, claims[0].UserId)
	assert.Equal(t, ExternalIdentityProviderOIDC, claims[1].Provider)
	assert.Equal(t, oidcUser.Id, claims[1].UserId)
}

func TestUpdateUserBindColumnRejectsExternalIdentityClaimedByAnotherUser(t *testing.T) {
	truncateTables(t)

	first := User{Username: "github-bind-one", Password: "password", AffCode: "github-bind-one"}
	second := User{Username: "github-bind-two", Password: "password", AffCode: "github-bind-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)

	require.NoError(t, UpdateUserBindColumn(first.Id, "github_id", "github-123"))
	err := UpdateUserBindColumn(second.Id, "github_id", "github-123")
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, second.Id).Error)
	assert.Empty(t, reloaded.GitHubId)
}

func TestUserCanBindDistinctOAuthChannelsWhenIdentitiesAreAvailable(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		firstColumn    string
		firstProvider  string
		firstSubject   string
		secondColumn   string
		secondProvider string
		secondSubject  string
		username       string
	}{
		{
			name: "GitHub then Google OIDC", firstColumn: "github_id", firstProvider: ExternalIdentityProviderGitHub, firstSubject: "github-first-channel",
			secondColumn: "oidc_id", secondProvider: ExternalIdentityProviderOIDC, secondSubject: "oidc-second-channel",
			username: "github-then-oidc",
		},
		{
			name: "Google OIDC then GitHub", firstColumn: "oidc_id", firstProvider: ExternalIdentityProviderOIDC, firstSubject: "oidc-first-channel",
			secondColumn: "github_id", secondProvider: ExternalIdentityProviderGitHub, secondSubject: "github-second-channel",
			username: "oidc-then-github",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			truncateTables(t)

			user := User{Username: testCase.username, Password: "password", AffCode: testCase.username}
			require.NoError(t, DB.Create(&user).Error)
			require.NoError(t, UpdateUserBindColumn(user.Id, testCase.firstColumn, testCase.firstSubject))
			require.NoError(t, UpdateUserBindColumn(user.Id, testCase.secondColumn, testCase.secondSubject))

			var reloaded User
			require.NoError(t, DB.First(&reloaded, user.Id).Error)
			assert.Equal(t, testCase.firstSubject, map[string]string{
				"github_id": reloaded.GitHubId,
				"oidc_id":   reloaded.OidcId,
			}[testCase.firstColumn])
			assert.Equal(t, testCase.secondSubject, map[string]string{
				"github_id": reloaded.GitHubId,
				"oidc_id":   reloaded.OidcId,
			}[testCase.secondColumn])

			for provider, subject := range map[string]string{
				testCase.firstProvider:  testCase.firstSubject,
				testCase.secondProvider: testCase.secondSubject,
			} {
				var claim ExternalIdentityClaim
				require.NoError(t, DB.Where("provider = ? AND subject = ?", provider, subject).First(&claim).Error)
				assert.Equal(t, user.Id, claim.UserId)
			}
		})
	}
}

func TestDifferentUsersCanUseDifferentOAuthChannels(t *testing.T) {
	truncateTables(t)

	githubUser := User{Username: "github-local-account", Password: "password", AffCode: "github-local-account"}
	oidcUser := User{Username: "oidc-local-account", Password: "password", AffCode: "oidc-local-account"}
	require.NoError(t, DB.Create(&githubUser).Error)
	require.NoError(t, DB.Create(&oidcUser).Error)

	require.NoError(t, UpdateUserBindColumn(githubUser.Id, "github_id", "github-independent"))
	require.NoError(t, UpdateUserBindColumn(oidcUser.Id, "oidc_id", "oidc-independent"))
}

func TestCustomAndBuiltInOAuthBindingsCanCoexist(t *testing.T) {
	t.Run("built-in then custom", func(t *testing.T) {
		truncateTables(t)

		user := User{Username: "built-in-before-custom", Password: "password", AffCode: "built-in-before-custom"}
		require.NoError(t, DB.Create(&user).Error)
		require.NoError(t, UpdateUserBindColumn(user.Id, "github_id", "github-before-custom"))

		require.NoError(t, CreateUserOAuthBinding(&UserOAuthBinding{
			UserId: user.Id, ProviderId: 101, ProviderUserId: "custom-second-channel",
		}))
		binding, err := GetUserOAuthBinding(user.Id, 101)
		require.NoError(t, err)
		assert.Equal(t, "custom-second-channel", binding.ProviderUserId)
		var reloaded User
		require.NoError(t, DB.First(&reloaded, user.Id).Error)
		assert.Equal(t, "github-before-custom", reloaded.GitHubId)
	})

	t.Run("custom then built-in", func(t *testing.T) {
		truncateTables(t)

		user := User{Username: "custom-before-built-in", Password: "password", AffCode: "custom-before-built-in"}
		require.NoError(t, DB.Create(&user).Error)
		require.NoError(t, CreateUserOAuthBinding(&UserOAuthBinding{
			UserId: user.Id, ProviderId: 202, ProviderUserId: "custom-first-channel",
		}))

		require.NoError(t, UpdateUserBindColumn(user.Id, "oidc_id", "oidc-second-channel"))
		binding, err := GetUserOAuthBinding(user.Id, 202)
		require.NoError(t, err)
		assert.Equal(t, "custom-first-channel", binding.ProviderUserId)
		var reloaded User
		require.NoError(t, DB.First(&reloaded, user.Id).Error)
		assert.Equal(t, "oidc-second-channel", reloaded.OidcId)
	})
}

func TestUpdateUserBindColumnRejectsSecondIdentityForSameProvider(t *testing.T) {
	truncateTables(t)

	user := User{Username: "github-owner", Password: "password", AffCode: "github-owner"}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, UpdateUserBindColumn(user.Id, "github_id", "github-original"))
	require.NoError(t, UpdateUserBindColumn(user.Id, "github_id", "github-original"))

	err := UpdateUserBindColumn(user.Id, "github_id", "github-replacement")
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, "github-original", reloaded.GitHubId)

	var claims []ExternalIdentityClaim
	require.NoError(t, DB.Where("provider = ? AND user_id = ?", ExternalIdentityProviderGitHub, user.Id).Find(&claims).Error)
	require.Len(t, claims, 1)
	assert.Equal(t, "github-original", claims[0].Subject)
}

func TestCustomOAuthBindingRejectsIdentityReuseAndRebinding(t *testing.T) {
	truncateTables(t)

	first := User{Username: "custom-owner-one", Password: "password", AffCode: "custom-owner-one"}
	second := User{Username: "custom-owner-two", Password: "password", AffCode: "custom-owner-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)
	require.NoError(t, UpdateUserOAuthBinding(first.Id, 303, "custom-original"))
	require.NoError(t, UpdateUserOAuthBinding(first.Id, 303, "custom-original"))

	err := UpdateUserOAuthBinding(first.Id, 303, "custom-replacement")
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)
	err = UpdateUserOAuthBinding(second.Id, 303, "custom-original")
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	binding, err := GetUserOAuthBinding(first.Id, 303)
	require.NoError(t, err)
	assert.Equal(t, "custom-original", binding.ProviderUserId)

	var secondBindings int64
	require.NoError(t, DB.Model(&UserOAuthBinding{}).Where("user_id = ?", second.Id).Count(&secondBindings).Error)
	assert.Zero(t, secondBindings)
}

func TestUpdateUserBindColumnRejectsMissingUserWithoutCreatingClaim(t *testing.T) {
	truncateTables(t)

	err := UpdateUserBindColumn(999999, "github_id", "github-missing-user")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var count int64
	require.NoError(t, DB.Model(&ExternalIdentityClaim{}).
		Where("provider = ? AND subject = ?", ExternalIdentityProviderGitHub, "github-missing-user").
		Count(&count).Error)
	assert.Zero(t, count)
}

func TestClearTelegramBindingReleasesIdentityClaim(t *testing.T) {
	truncateTables(t)

	user := User{Username: "telegram-unbind", Password: "password", TelegramId: "telegram-unbind-id"}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, user.TelegramId, user.Id)
	}))

	require.NoError(t, user.ClearBinding(ExternalIdentityProviderTelegram))
	assert.Empty(t, user.TelegramId)

	var count int64
	require.NoError(t, DB.Model(&ExternalIdentityClaim{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestClearGitHubAndOIDCBindingsReleaseIdentityClaims(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		bindingType string
		provider    string
		column      string
		subject     string
	}{
		{name: "GitHub", bindingType: "github", provider: ExternalIdentityProviderGitHub, column: "github_id", subject: "github-unbind-id"},
		{name: "OIDC", bindingType: "oidc", provider: ExternalIdentityProviderOIDC, column: "oidc_id", subject: "oidc-unbind-id"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			truncateTables(t)

			user := User{Username: "oauth-unbind-" + testCase.bindingType, Password: "password", AffCode: "oauth-unbind-" + testCase.bindingType}
			require.NoError(t, DB.Create(&user).Error)
			require.NoError(t, UpdateUserBindColumn(user.Id, testCase.column, testCase.subject))

			require.NoError(t, user.ClearBinding(testCase.bindingType))

			var count int64
			require.NoError(t, DB.Model(&ExternalIdentityClaim{}).
				Where("provider = ? AND user_id = ?", testCase.provider, user.Id).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestInitializeExternalIdentityClaimsIsIdempotent(t *testing.T) {
	truncateTables(t)

	user := User{
		Username:   "external-identity-legacy",
		Password:   "password",
		TelegramId: "telegram-legacy-id",
		GitHubId:   "github-legacy-id",
		OidcId:     "oidc-legacy-id",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, InitializeExternalIdentityClaims())
	require.NoError(t, InitializeExternalIdentityClaims())

	for provider, subject := range map[string]string{
		ExternalIdentityProviderTelegram: user.TelegramId,
		ExternalIdentityProviderGitHub:   user.GitHubId,
		ExternalIdentityProviderOIDC:     user.OidcId,
	} {
		var claim ExternalIdentityClaim
		require.NoError(t, DB.Where("provider = ? AND subject = ?", provider, subject).First(&claim).Error)
		assert.Equal(t, user.Id, claim.UserId)
	}
}

func TestUpdateGitHubIdReplacesLegacyIdentityClaim(t *testing.T) {
	truncateTables(t)

	user := User{Username: "github-legacy-migration", Password: "password", GitHubId: "legacy-login"}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderGitHub, user.GitHubId, user.Id)
	}))

	require.NoError(t, user.UpdateGitHubId("123456"))

	var claims []ExternalIdentityClaim
	require.NoError(t, DB.Where("provider = ? AND user_id = ?", ExternalIdentityProviderGitHub, user.Id).Find(&claims).Error)
	require.Len(t, claims, 1)
	assert.Equal(t, "123456", claims[0].Subject)
	assert.Equal(t, "123456", user.GitHubId)
}

func TestInitializeExternalIdentityClaimsRejectsAmbiguousLegacyBindings(t *testing.T) {
	truncateTables(t)

	first := User{Username: "telegram-legacy-one", Password: "password", TelegramId: "duplicate-telegram-id", AffCode: "telegram-legacy-one"}
	second := User{Username: "telegram-legacy-two", Password: "password", TelegramId: "duplicate-telegram-id", AffCode: "telegram-legacy-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)

	err := InitializeExternalIdentityClaims()
	assert.ErrorIs(t, err, ErrExternalIdentityAlreadyClaimed)

	var count int64
	require.NoError(t, DB.Model(&ExternalIdentityClaim{}).Count(&count).Error)
	assert.Zero(t, count)
}
