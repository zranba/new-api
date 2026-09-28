package model

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// NextTokenQuotaReset uses calendar dates in the server's location, including
// short months and DST. Weekly days are ISO weekdays (Monday=1, Sunday=7).
func NextTokenQuotaReset(now time.Time, period string, day int, clock string) (int64, error) {
	if period == "" || period == "never" {
		return 0, nil
	}
	wall, err := time.Parse("15:04", clock)
	if err != nil || len(clock) != 5 {
		return 0, errors.New("invalid quota reset time")
	}
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case "daily":
		if day != 0 {
			return 0, errors.New("daily quota reset day must be zero")
		}
	case "weekly":
		if day < 1 || day > 7 {
			return 0, errors.New("invalid quota reset weekday")
		}
		days := (day%7 - int(date.Weekday()) + 7) % 7
		date = date.AddDate(0, 0, days)
	case "monthly":
		if day < 1 || day > 31 {
			return 0, errors.New("invalid quota reset month day")
		}
		date = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return 0, errors.New("invalid quota reset period")
	}
	for {
		candidateDate := date
		if period == "monthly" {
			lastDay := date.AddDate(0, 1, -1).Day()
			candidateDate = date.AddDate(0, 0, min(day, lastDay)-1)
		}
		next := tokenResetOnCalendarDay(candidateDate, wall.Hour(), wall.Minute(), now.Location())
		if next.After(now) {
			return next.Unix(), nil
		}
		switch period {
		case "daily":
			date = date.AddDate(0, 0, 1)
		case "weekly":
			date = date.AddDate(0, 0, 7)
		case "monthly":
			date = date.AddDate(0, 1, 0)
		}
	}
}

// On a DST transition date, select the first occurrence of the requested wall
// time, or the first valid wall minute after a skipped time. Ordinary dates
// take the direct time.Date path. A skipped calendar date has no occurrence.
func tokenResetOnCalendarDay(date time.Time, hour, minute int, location *time.Location) time.Time {
	candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location)
	_, beforeOffset := candidate.Add(-24 * time.Hour).Zone()
	_, afterOffset := candidate.Add(24 * time.Hour).Zone()
	if beforeOffset == afterOffset && candidate.Year() == date.Year() && candidate.Month() == date.Month() &&
		candidate.Day() == date.Day() && candidate.Hour() == hour && candidate.Minute() == minute {
		return candidate
	}
	// Search real instants rather than adding wall hours, which is ambiguous at
	// a clock change. This window covers every supported UTC offset.
	end := date.Add(50 * time.Hour)
	for instant := date.Add(-26 * time.Hour); instant.Before(end); instant = instant.Add(time.Minute) {
		local := instant.In(location)
		if local.Year() == date.Year() && local.Month() == date.Month() && local.Day() == date.Day() &&
			local.Hour()*60+local.Minute() >= hour*60+minute {
			return local
		}
	}
	return time.Time{}
}

func (token *Token) QuotaResetDue(now int64) bool {
	return token.QuotaResetPeriod != "" && token.QuotaResetPeriod != "never" &&
		!token.UnlimitedQuota && token.QuotaResetAmount > 0 &&
		token.QuotaResetNextTime > 0 && token.QuotaResetNextTime <= now &&
		token.Status != common.TokenStatusExpired &&
		(token.ExpiredTime == -1 || token.ExpiredTime > now)
}

// ResetTokenQuotaIfDue is shared by the scheduler and request admission. The
// conditional update also provides compare-and-swap protection on SQLite,
// where lockForUpdate intentionally does not emit FOR UPDATE.
func ResetTokenQuotaIfDue(id int, now int64) (*Token, bool, error) {
	var token Token
	reset := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&token, id).Error; err != nil {
			return err
		}
		if !token.QuotaResetDue(now) {
			return nil
		}
		if token.QuotaEpoch == math.MaxInt64 || token.QuotaResetAmount > common.MaxWalletQuota {
			return errors.New("token quota reset exceeds supported range")
		}
		next, err := NextTokenQuotaReset(time.Unix(now, 0).In(time.Local), token.QuotaResetPeriod, token.QuotaResetDay, token.QuotaResetTime)
		if err != nil {
			return err
		}
		if err := invalidateTokenCacheForMutation(token.Key); err != nil {
			return err
		}
		status := token.Status
		if status == common.TokenStatusExhausted {
			status = common.TokenStatusEnabled
		}
		result := tx.Model(&Token{}).
			Where("id = ? AND quota_epoch = ? AND quota_reset_next_time = ?", id, token.QuotaEpoch, token.QuotaResetNextTime).
			Updates(map[string]any{
				"remain_quota": token.QuotaResetAmount, "status": status,
				"quota_epoch": token.QuotaEpoch + 1, "quota_reset_last_time": now, "quota_reset_next_time": next,
			})
		if result.Error != nil {
			return result.Error
		}
		reset = result.RowsAffected == 1
		return tx.First(&token, id).Error
	})
	if err != nil {
		return nil, false, err
	}
	if reset {
		if err := invalidateTokenCacheForMutation(token.Key); err != nil {
			common.SysError(fmt.Sprintf("failed to invalidate reset token cache (id=%d): %v", id, err))
		}
		common.SysLog(fmt.Sprintf("token quota reset: user_id=%d token_id=%d epoch=%d next_reset=%d", token.UserId, id, token.QuotaEpoch, token.QuotaResetNextTime))
	}
	return &token, reset, nil
}

// DueTokenQuotaResets uses keyset pagination so a failed or concurrently changed
// token cannot keep the maintenance job looping on the same batch.
func DueTokenQuotaResets(afterID int, now int64, limit int) ([]Token, error) {
	var tokens []Token
	err := DB.Select("id").Where("id > ? AND quota_reset_next_time > 0 AND quota_reset_next_time <= ?", afterID, now).
		Where("quota_reset_period IN ? AND quota_reset_amount > 0", []string{"daily", "weekly", "monthly"}).
		Where("unlimited_quota = ? AND status <> ?", false, common.TokenStatusExpired).
		Where("expired_time = ? OR expired_time > ?", -1, now).
		Order("id").Limit(limit).Find(&tokens).Error
	return tokens, err
}

// A late settlement always contributes to lifetime usage, but can only change
// the remaining balance of the epoch in which its request was admitted. Legacy
// batch records belong to epoch zero, including records queued before reset.
func applyTokenQuotaDelta(id, delta int, epoch int64) error {
	result := DB.Model(&Token{}).Where("id = ?", id).Updates(map[string]any{
		"remain_quota":  gorm.Expr("CASE WHEN quota_epoch = ? THEN remain_quota + ? ELSE remain_quota END", epoch, delta),
		"used_quota":    gorm.Expr("used_quota - ?", delta),
		"accessed_time": common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 && delta != 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
