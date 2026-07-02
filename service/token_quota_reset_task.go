package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	tokenQuotaResetTickInterval = 1 * time.Minute
	tokenQuotaResetBatchSize    = 300
)

var (
	tokenQuotaResetOnce    sync.Once
	tokenQuotaResetRunning atomic.Bool
)

func StartTokenQuotaResetTask() {
	tokenQuotaResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("token quota reset task started: tick=%s", tokenQuotaResetTickInterval))
			ticker := time.NewTicker(tokenQuotaResetTickInterval)
			defer ticker.Stop()

			runTokenQuotaResetOnce()
			for range ticker.C {
				runTokenQuotaResetOnce()
			}
		})
	})
}

func runTokenQuotaResetOnce() {
	if !tokenQuotaResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer tokenQuotaResetRunning.Store(false)

	ctx := context.Background()
	totalReset := 0
	for {
		n, err := model.ResetDueTokens(tokenQuotaResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("token quota reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < tokenQuotaResetBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalReset > 0 {
		logger.LogDebug(ctx, "token quota reset task: reset_count=%d", totalReset)
	}
}
