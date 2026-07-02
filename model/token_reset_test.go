package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenResetTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLOGDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Token{}))

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLOGDB
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		clearPendingTokenQuotaDelta(1)
		clearPendingTokenQuotaDelta(2)
		clearPendingTokenQuotaDelta(3)
	})

	return db
}

func TestCalcNextTokenQuotaResetTimeAlignsToLocalBoundaries(t *testing.T) {
	base := time.Date(2026, 7, 1, 13, 15, 30, 0, time.Local)

	assert.Equal(t,
		time.Date(2026, 7, 2, 0, 0, 0, 0, time.Local).Unix(),
		CalcNextTokenQuotaResetTime(base, TokenQuotaResetDaily),
	)
	assert.Equal(t,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local).Unix(),
		CalcNextTokenQuotaResetTime(base, TokenQuotaResetMonthly),
	)
	assert.Zero(t, CalcNextTokenQuotaResetTime(base, "weekly"))
}

func TestBackfillTokenQuotaResetAmountUsesCurrentTotal(t *testing.T) {
	db := setupTokenResetTestDB(t)
	require.NoError(t, db.Create(&Token{
		UserId:           1,
		Key:              "backfill-limited",
		Status:           common.TokenStatusEnabled,
		Name:             "limited",
		ExpiredTime:      -1,
		RemainQuota:      25,
		UsedQuota:        75,
		UnlimitedQuota:   false,
		QuotaResetAmount: 0,
	}).Error)
	require.NoError(t, db.Create(&Token{
		UserId:           1,
		Key:              "backfill-unlimited",
		Status:           common.TokenStatusEnabled,
		Name:             "unlimited",
		ExpiredTime:      -1,
		RemainQuota:      0,
		UsedQuota:        75,
		UnlimitedQuota:   true,
		QuotaResetAmount: 0,
	}).Error)

	require.NoError(t, backfillTokenQuotaResetAmount(true))

	var limited Token
	require.NoError(t, db.First(&limited, "key = ?", "backfill-limited").Error)
	assert.Equal(t, 100, limited.QuotaResetAmount)

	var unlimited Token
	require.NoError(t, db.First(&unlimited, "key = ?", "backfill-unlimited").Error)
	assert.Zero(t, unlimited.QuotaResetAmount)
}

func TestBackfillTokenQuotaResetAmountSkipsWhenColumnAlreadyExisted(t *testing.T) {
	db := setupTokenResetTestDB(t)
	require.NoError(t, db.Create(&Token{
		UserId:           1,
		Key:              "backfill-skip",
		Status:           common.TokenStatusEnabled,
		Name:             "limited",
		ExpiredTime:      -1,
		RemainQuota:      25,
		UsedQuota:        75,
		UnlimitedQuota:   false,
		QuotaResetAmount: 0,
	}).Error)

	require.NoError(t, backfillTokenQuotaResetAmount(false))

	var token Token
	require.NoError(t, db.First(&token, "key = ?", "backfill-skip").Error)
	assert.Zero(t, token.QuotaResetAmount)
}

func TestResetTokenQuotaRestoresAmountAndRevivesExhaustedToken(t *testing.T) {
	db := setupTokenResetTestDB(t)
	now := GetDBTimestamp()
	accessedTime := now - 3600
	token := &Token{
		UserId:             1,
		Key:                "manual-reset",
		Status:             common.TokenStatusExhausted,
		Name:               "manual",
		ExpiredTime:        -1,
		RemainQuota:        0,
		UsedQuota:          100,
		UnlimitedQuota:     false,
		QuotaResetAmount:   500,
		QuotaResetPeriod:   TokenQuotaResetDaily,
		NextQuotaResetTime: now - 60,
		AccessedTime:       accessedTime,
	}
	require.NoError(t, db.Create(token).Error)
	addNewRecord(BatchUpdateTypeTokenQuota, token.Id, -50)

	resetToken, err := ResetTokenQuota(token.Id, token.UserId)
	require.NoError(t, err)

	assert.Equal(t, 500, resetToken.RemainQuota)
	assert.Zero(t, resetToken.UsedQuota)
	assert.Equal(t, common.TokenStatusEnabled, resetToken.Status)
	assert.Greater(t, resetToken.LastQuotaResetTime, int64(0))
	assert.Greater(t, resetToken.NextQuotaResetTime, resetToken.LastQuotaResetTime)
	assert.Equal(t, accessedTime, resetToken.AccessedTime)

	batchUpdateLocks[BatchUpdateTypeTokenQuota].Lock()
	_, pending := batchUpdateStores[BatchUpdateTypeTokenQuota][token.Id]
	batchUpdateLocks[BatchUpdateTypeTokenQuota].Unlock()
	assert.False(t, pending)
}

func TestResetTokenQuotaKeepsDisabledTokenDisabled(t *testing.T) {
	db := setupTokenResetTestDB(t)
	token := &Token{
		UserId:           1,
		Key:              "disabled-reset",
		Status:           common.TokenStatusDisabled,
		Name:             "disabled",
		ExpiredTime:      -1,
		RemainQuota:      0,
		UsedQuota:        100,
		UnlimitedQuota:   false,
		QuotaResetAmount: 500,
		QuotaResetPeriod: TokenQuotaResetMonthly,
	}
	require.NoError(t, db.Create(token).Error)

	resetToken, err := ResetTokenQuota(token.Id, token.UserId)
	require.NoError(t, err)

	assert.Equal(t, 500, resetToken.RemainQuota)
	assert.Zero(t, resetToken.UsedQuota)
	assert.Equal(t, common.TokenStatusDisabled, resetToken.Status)
}

func TestResetDueTokensSkipsExpiredTokenAndStopsSchedule(t *testing.T) {
	db := setupTokenResetTestDB(t)
	now := GetDBTimestamp()
	token := &Token{
		UserId:             1,
		Key:                "expired-due",
		Status:             common.TokenStatusExpired,
		Name:               "expired",
		ExpiredTime:        now - 1,
		RemainQuota:        0,
		UsedQuota:          100,
		UnlimitedQuota:     false,
		QuotaResetAmount:   500,
		QuotaResetPeriod:   TokenQuotaResetDaily,
		NextQuotaResetTime: now - 60,
	}
	require.NoError(t, db.Create(token).Error)

	resetCount, err := ResetDueTokens(10)
	require.NoError(t, err)
	assert.Zero(t, resetCount)

	var got Token
	require.NoError(t, db.First(&got, token.Id).Error)
	assert.Zero(t, got.RemainQuota)
	assert.Equal(t, 100, got.UsedQuota)
	assert.Zero(t, got.NextQuotaResetTime)
	assert.Equal(t, common.TokenStatusExpired, got.Status)
}

func TestResetDueTokensPreservesAccessedTime(t *testing.T) {
	db := setupTokenResetTestDB(t)
	now := GetDBTimestamp()
	accessedTime := now - 7200
	token := &Token{
		UserId:             1,
		Key:                "due-reset",
		Status:             common.TokenStatusExhausted,
		Name:               "due",
		ExpiredTime:        -1,
		RemainQuota:        0,
		UsedQuota:          100,
		UnlimitedQuota:     false,
		QuotaResetAmount:   500,
		QuotaResetPeriod:   TokenQuotaResetDaily,
		NextQuotaResetTime: now - 60,
		AccessedTime:       accessedTime,
	}
	require.NoError(t, db.Create(token).Error)

	resetCount, err := ResetDueTokens(10)
	require.NoError(t, err)
	assert.Equal(t, 1, resetCount)

	var got Token
	require.NoError(t, db.First(&got, token.Id).Error)
	assert.Equal(t, 500, got.RemainQuota)
	assert.Zero(t, got.UsedQuota)
	assert.Equal(t, common.TokenStatusEnabled, got.Status)
	assert.Greater(t, got.LastQuotaResetTime, int64(0))
	assert.Equal(t, accessedTime, got.AccessedTime)
}

func TestValidateUserTokenLazilyResetsDueExhaustedToken(t *testing.T) {
	db := setupTokenResetTestDB(t)
	now := GetDBTimestamp()
	accessedTime := now - 1800
	token := &Token{
		UserId:             1,
		Key:                "lazy-reset",
		Status:             common.TokenStatusExhausted,
		Name:               "lazy",
		ExpiredTime:        -1,
		RemainQuota:        0,
		UsedQuota:          100,
		UnlimitedQuota:     false,
		QuotaResetAmount:   500,
		QuotaResetPeriod:   TokenQuotaResetDaily,
		NextQuotaResetTime: now - 60,
		AccessedTime:       accessedTime,
	}
	require.NoError(t, db.Create(token).Error)

	got, err := ValidateUserToken(token.Key)
	require.NoError(t, err)

	assert.Equal(t, common.TokenStatusEnabled, got.Status)
	assert.Equal(t, 500, got.RemainQuota)
	assert.Zero(t, got.UsedQuota)
	assert.Equal(t, accessedTime, got.AccessedTime)
}
