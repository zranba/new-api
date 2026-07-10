package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tokenQuotaResetResponseItem struct {
	ID                 int    `json:"id"`
	Key                string `json:"key"`
	Status             int    `json:"status"`
	RemainQuota        int    `json:"remain_quota"`
	UsedQuota          int    `json:"used_quota"`
	QuotaResetAmount   int    `json:"quota_reset_amount"`
	QuotaResetPeriod   string `json:"quota_reset_period"`
	NextQuotaResetTime int64  `json:"next_quota_reset_time"`
	LastQuotaResetTime int64  `json:"last_quota_reset_time"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
}

func decodeTokenQuotaResetItem(t *testing.T, recorder *httptest.ResponseRecorder) tokenQuotaResetResponseItem {
	t.Helper()
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, "unexpected response message: %s", response.Message)
	var detail tokenQuotaResetResponseItem
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	return detail
}

func TestAddTokenDefaultsResetAmountToInitialQuota(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	body := map[string]any{
		"name":                 "reset-default",
		"expired_time":         -1,
		"remain_quota":         300,
		"unlimited_quota":      false,
		"quota_reset_period":   model.TokenQuotaResetDaily,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, "unexpected response message: %s", response.Message)

	var token model.Token
	require.NoError(t, db.First(&token, "name = ?", "reset-default").Error)
	assert.Equal(t, 300, token.QuotaResetAmount)
	assert.Equal(t, model.TokenQuotaResetDaily, token.QuotaResetPeriod)
	assert.Greater(t, token.NextQuotaResetTime, int64(0))
}

func TestUpdateTokenPreservesResetFieldsWhenOldClientOmitsThem(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "preserve-reset", "preserve-reset-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"remain_quota":          100,
		"unlimited_quota":       false,
		"quota_reset_amount":    500,
		"quota_reset_period":    model.TokenQuotaResetDaily,
		"next_quota_reset_time": model.CalcNextTokenQuotaResetTime(time.Now(), model.TokenQuotaResetDaily),
	}).Error)

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "preserve-reset-updated",
		"expired_time":         -1,
		"remain_quota":         200,
		"unlimited_quota":      false,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, "unexpected response message: %s", response.Message)

	var updated model.Token
	require.NoError(t, db.First(&updated, token.Id).Error)
	assert.Equal(t, 500, updated.QuotaResetAmount)
	assert.Equal(t, model.TokenQuotaResetDaily, updated.QuotaResetPeriod)
	assert.Greater(t, updated.NextQuotaResetTime, int64(0))
}

func TestUpdateTokenCanEnableExhaustedTokenWhenQuotaIsRestored(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "exhausted-update", "exhausted-update-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"status":          common.TokenStatusExhausted,
		"remain_quota":    0,
		"used_quota":      100,
		"unlimited_quota": false,
	}).Error)

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "exhausted-update",
		"status":               common.TokenStatusEnabled,
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      false,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, "unexpected response message: %s", response.Message)

	var updated model.Token
	require.NoError(t, db.First(&updated, token.Id).Error)
	assert.Equal(t, common.TokenStatusEnabled, updated.Status)
	assert.Equal(t, 100, updated.RemainQuota)
}

func TestResetTokenQuotaRequiresOwnershipAndMasksResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "manual-reset", "manual-reset-raw-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"status":             common.TokenStatusExhausted,
		"remain_quota":       0,
		"used_quota":         100,
		"unlimited_quota":    false,
		"quota_reset_amount": 500,
		"quota_reset_period": model.TokenQuotaResetMonthly,
	}).Error)

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/quota/reset", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	ResetTokenQuota(unauthorizedCtx)
	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	require.False(t, unauthorizedResponse.Success)

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/quota/reset", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	ResetTokenQuota(authorizedCtx)

	detail := decodeTokenQuotaResetItem(t, authorizedRecorder)
	assert.Equal(t, 500, detail.RemainQuota)
	assert.Zero(t, detail.UsedQuota)
	assert.Equal(t, common.TokenStatusEnabled, detail.Status)
	assert.Equal(t, token.GetMaskedKey(), detail.Key)
	assert.NotContains(t, authorizedRecorder.Body.String(), token.Key)
	assert.False(t, strings.Contains(authorizedRecorder.Body.String(), "manual-reset-raw-key"))
}

func TestResetTokenQuotaRejectsUnlimitedToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "unlimited-reset", "unlimited-reset-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"unlimited_quota":    true,
		"quota_reset_amount": 500,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/quota/reset", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	ResetTokenQuota(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "Unlimited tokens do not need quota reset", response.Message)
}

func TestResetTokenQuotaRejectsMissingResetAmount(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "missing-reset-amount", "missing-reset-amount-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"remain_quota":          20,
		"used_quota":            80,
		"unlimited_quota":       false,
		"quota_reset_amount":    0,
		"quota_reset_period":    model.TokenQuotaResetDaily,
		"last_quota_reset_time": 0,
		"next_quota_reset_time": model.CalcNextTokenQuotaResetTime(time.Now(), model.TokenQuotaResetDaily),
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/quota/reset", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	ResetTokenQuota(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "Please set the token reset amount first", response.Message)
}

func TestResetTokenQuotaRejectsExpiredToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "expired-reset", "expired-reset-key")
	require.NoError(t, db.Model(token).Updates(map[string]interface{}{
		"status":             common.TokenStatusExpired,
		"expired_time":       common.GetTimestamp() - 60,
		"remain_quota":       20,
		"used_quota":         80,
		"unlimited_quota":    false,
		"quota_reset_amount": 500,
		"quota_reset_period": model.TokenQuotaResetDaily,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/quota/reset", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	ResetTokenQuota(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "Token has expired and cannot be reset. Please modify the expiration time or set it to never expire", response.Message)

	var updated model.Token
	require.NoError(t, db.First(&updated, token.Id).Error)
	assert.Equal(t, 20, updated.RemainQuota)
	assert.Equal(t, 80, updated.UsedQuota)
	assert.Equal(t, common.TokenStatusExpired, updated.Status)
}
