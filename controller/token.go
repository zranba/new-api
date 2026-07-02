package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

func buildMaskedTokenResponse(token *model.Token) *model.Token {
	if token == nil {
		return nil
	}
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	return &maskedToken
}

func buildMaskedTokenResponses(tokens []*model.Token) []*model.Token {
	maskedTokens := make([]*model.Token, 0, len(tokens))
	for _, token := range tokens {
		maskedTokens = append(maskedTokens, buildMaskedTokenResponse(token))
	}
	return maskedTokens
}

type tokenPayloadFields struct {
	QuotaResetAmount bool
	QuotaResetPeriod bool
}

func parseTokenPayload(c *gin.Context) (model.Token, tokenPayloadFields, error) {
	var token model.Token
	var fields tokenPayloadFields
	rawBody, err := c.GetRawData()
	if err != nil {
		return token, fields, err
	}
	if err := common.Unmarshal(rawBody, &token); err != nil {
		return token, fields, err
	}
	var raw map[string]any
	if err := common.Unmarshal(rawBody, &raw); err == nil {
		_, fields.QuotaResetAmount = raw["quota_reset_amount"]
		_, fields.QuotaResetPeriod = raw["quota_reset_period"]
	}
	return token, fields, nil
}

func maxTokenQuotaValue() int {
	return int(1000000000 * common.QuotaPerUnit)
}

func normalizeTokenQuotaResetForCreate(c *gin.Context, token *model.Token, fields tokenPayloadFields) bool {
	if token.UnlimitedQuota {
		token.QuotaResetAmount = 0
		token.QuotaResetPeriod = model.TokenQuotaResetNever
		token.LastQuotaResetTime = 0
		token.NextQuotaResetTime = 0
		return true
	}
	if token.RemainQuota < 0 {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
		return false
	}
	maxQuotaValue := maxTokenQuotaValue()
	if token.RemainQuota > maxQuotaValue {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
		return false
	}
	token.QuotaResetPeriod = model.NormalizeTokenQuotaResetPeriod(token.QuotaResetPeriod)
	if !fields.QuotaResetAmount {
		token.QuotaResetAmount = token.RemainQuota
	}
	if token.QuotaResetAmount < 0 {
		common.ApiErrorMsg(c, "令牌重置额度不能为负数")
		return false
	}
	if token.QuotaResetAmount > maxQuotaValue {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
		return false
	}
	if token.QuotaResetPeriod != model.TokenQuotaResetNever && token.QuotaResetAmount <= 0 {
		common.ApiErrorMsg(c, "启用周期重置时必须设置大于 0 的重置额度")
		return false
	}
	token.LastQuotaResetTime = 0
	token.NextQuotaResetTime = model.CalcNextTokenQuotaResetTime(time.Now(), token.QuotaResetPeriod)
	return true
}

func normalizeTokenQuotaResetForUpdate(c *gin.Context, token *model.Token, cleanToken *model.Token, fields tokenPayloadFields) bool {
	if token.UnlimitedQuota {
		cleanToken.QuotaResetAmount = 0
		cleanToken.QuotaResetPeriod = model.TokenQuotaResetNever
		cleanToken.LastQuotaResetTime = 0
		cleanToken.NextQuotaResetTime = 0
		return true
	}
	if token.RemainQuota < 0 {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
		return false
	}
	maxQuotaValue := maxTokenQuotaValue()
	if token.RemainQuota > maxQuotaValue {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
		return false
	}
	resetAmount := cleanToken.QuotaResetAmount
	if fields.QuotaResetAmount {
		resetAmount = token.QuotaResetAmount
	} else if resetAmount <= 0 {
		resetAmount = token.RemainQuota
	}
	if resetAmount < 0 {
		common.ApiErrorMsg(c, "令牌重置额度不能为负数")
		return false
	}
	if resetAmount > maxQuotaValue {
		common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
		return false
	}
	resetPeriod := cleanToken.QuotaResetPeriod
	if fields.QuotaResetPeriod {
		resetPeriod = token.QuotaResetPeriod
	}
	resetPeriod = model.NormalizeTokenQuotaResetPeriod(resetPeriod)
	if resetPeriod != model.TokenQuotaResetNever && resetAmount <= 0 {
		common.ApiErrorMsg(c, "启用周期重置时必须设置大于 0 的重置额度")
		return false
	}
	cleanToken.QuotaResetAmount = resetAmount
	cleanToken.QuotaResetPeriod = resetPeriod
	if fields.QuotaResetAmount || fields.QuotaResetPeriod {
		cleanToken.NextQuotaResetTime = model.CalcNextTokenQuotaResetTime(time.Now(), resetPeriod)
	}
	if resetPeriod == model.TokenQuotaResetNever {
		cleanToken.NextQuotaResetTime = 0
	}
	return true
}

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := common.GetPageQuery(c)

	tokens, total, err := model.SearchUserTokens(userId, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := model.GetTokenByKey(strings.TrimPrefix(tokenKey, "sk-"), false)
	if err != nil {
		common.SysError("failed to get token by key: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":                "token_usage",
			"name":                  token.Name,
			"total_granted":         token.RemainQuota + token.UsedQuota,
			"total_used":            token.UsedQuota,
			"total_available":       token.RemainQuota,
			"unlimited_quota":       token.UnlimitedQuota,
			"quota_reset_amount":    token.QuotaResetAmount,
			"quota_reset_period":    model.NormalizeTokenQuotaResetPeriod(token.QuotaResetPeriod),
			"last_quota_reset_time": token.LastQuotaResetTime,
			"next_quota_reset_time": token.NextQuotaResetTime,
			"model_limits":          token.GetModelLimitsMap(),
			"model_limits_enabled":  token.ModelLimitsEnabled,
			"expires_at":            expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	token, fields, err := parseTokenPayload(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !normalizeTokenQuotaResetForCreate(c, &token, fields) {
		return
	}
	// 检查用户令牌数量是否已达上限
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := model.Token{
		UserId:             c.GetInt("id"),
		Name:               token.Name,
		Key:                key,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		QuotaResetAmount:   token.QuotaResetAmount,
		QuotaResetPeriod:   token.QuotaResetPeriod,
		LastQuotaResetTime: token.LastQuotaResetTime,
		NextQuotaResetTime: token.NextQuotaResetTime,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		AllowIps:           token.AllowIps,
		Group:              token.Group,
		CrossGroupRetry:    token.CrossGroupRetry,
	}
	err = cleanToken.Insert()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	statusOnly := c.Query("status_only")
	token, fields, err := parseTokenPayload(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status == common.TokenStatusEnabled {
		if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota &&
			!token.UnlimitedQuota && token.RemainQuota <= 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		if !normalizeTokenQuotaResetForUpdate(c, &token, cleanToken, fields) {
			return
		}
		// If you add more fields, please also update token.Update()
		if token.Status != 0 {
			cleanToken.Status = token.Status
		}
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = token.Group
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
	}
	err = cleanToken.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(cleanToken),
	})
}

func ResetTokenQuota(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.ResetTokenQuota(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userId := c.GetInt("id")
	tokens, err := model.GetTokenKeysByIds(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keysMap := make(map[int]string)
	for _, t := range tokens {
		keysMap[t.Id] = t.GetFullKey()
	}
	common.ApiSuccess(c, gin.H{"keys": keysMap})
}
