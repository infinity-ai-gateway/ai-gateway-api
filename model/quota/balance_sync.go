package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
)

// BalanceSyncManager 配额余额同步管理器
type BalanceSyncManager struct {
	txn            itxn.TxnStorager
	apiKeyStorager api_key.APIKeyStorager
	planStorager   QuotaPlanStorager
	entityStorager entity.EntityStorager
	quotaCache     quotacache.QuotaCache
	clock          Clock
}

// NewBalanceSyncManager 创建配额余额同步管理器
func NewBalanceSyncManager(
	txn itxn.TxnStorager,
	apiKeyStorager api_key.APIKeyStorager,
	planStorager QuotaPlanStorager,
	entityStorager entity.EntityStorager,
	quotaCache quotacache.QuotaCache,
	clock Clock,
) *BalanceSyncManager {
	if clock == nil {
		clock = NewRealClock()
	}
	return &BalanceSyncManager{
		txn:            txn,
		apiKeyStorager: apiKeyStorager,
		planStorager:   planStorager,
		entityStorager: entityStorager,
		quotaCache:     quotaCache,
		clock:          clock,
	}
}

// ResetExpiredBalances 检查并重置所有到期的配额计划
func (m *BalanceSyncManager) ResetExpiredBalances(ctx context.Context) error {
	return m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		// 1. 获取所有需要重置的配额计划（非无限配额且设置了重置周期）
		plans, err := m.planStorager.FetchQuotaPlanList(ctx, &QuotaPlanFilter{
			ResetPeriod: []string{"weekly", "monthly"},
			Unlimited:   lib.PBool(false),
		})
		if err != nil {
			return err
		}

		now := m.clock.Now()

		// 2. 遍历每个配额计划
		for _, plan := range plans {
			if plan.ID == nil || plan.ResetPeriod == nil {
				continue
			}

			// 3. 判断是否需要重置（基于自然周/自然月）
			shouldReset := m.shouldResetByPeriod(plan.LastResetAt, *plan.ResetPeriod, now)

			if shouldReset {
				// 4. 重置该配额计划下所有 API-Key / Entity 的 Redis 使用量
				if err := m.resetAPIKeysRedisUsage(ctx, *plan.ID); err != nil {
					fmt.Printf("Failed to reset Redis usage for plan %d: %v\n", *plan.ID, err)
					continue
				}

				// 5. 条件更新 quota_plans.last_reset_at，作为同一周期内幂等重置的兜底
				periodStart := m.getPeriodStart(*plan.ResetPeriod, now)
				_, err = m.planStorager.UpdateQuotaPlan(ctx, &QuotaPlanFilter{
					ID:                plan.ID,
					LastResetAtBefore: &periodStart,
				}, &QuotaPlanParam{
					LastResetAt: lib.PTime(now),
				})
				if err != nil {
					fmt.Printf("Failed to update last_reset_at for plan %d: %v\n", *plan.ID, err)
				} else {
					fmt.Printf("Reset balance for plan %d at %v\n", *plan.ID, now)
				}
			}
		}

		return nil
	})
}

// shouldResetByPeriod 判断是否应该根据周期重置配额
// 采用自然周/自然月的边界判断
func (m *BalanceSyncManager) shouldResetByPeriod(lastResetAt *time.Time, resetPeriod string, now time.Time) bool {
	if resetPeriod != "weekly" && resetPeriod != "monthly" {
		return false
	}
	currentStart := m.getPeriodStart(resetPeriod, now)
	if lastResetAt == nil {
		return true // 从未重置过，需要重置
	}
	lastStart := m.getPeriodStart(resetPeriod, *lastResetAt)
	return currentStart.After(lastStart)
}

// getPeriodStart 返回给定时间所在周期的开始时间
func (m *BalanceSyncManager) getPeriodStart(resetPeriod string, t time.Time) time.Time {
	switch resetPeriod {
	case "weekly":
		return getWeekStart(t)
	case "monthly":
		return getMonthStart(t)
	default:
		return t
	}
}

// getWeekStart 获取给定时间所在周的周一 00:00:00
func getWeekStart(t time.Time) time.Time {
	weekday := t.Weekday()
	// Go 的 Weekday() 返回 0=Sunday, 1=Monday, ..., 6=Saturday
	// 转换为 Monday=0, Tuesday=1, ..., Sunday=6
	if weekday == time.Sunday {
		weekday = 6
	} else {
		weekday -= 1
	}

	// 回退到周一
	weekStart := t.AddDate(0, 0, -int(weekday))
	// 设置为 00:00:00
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, t.Location())
}

// getMonthStart 获取给定时间所在月的1日 00:00:00
func getMonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// resetAPIKeysRedisUsage 重置指定配额计划下所有 API-Key 和 Entity 的 Redis 使用量
// 将 Redis key 的值重置为配额总量
func (m *BalanceSyncManager) resetAPIKeysRedisUsage(ctx context.Context, planID int64) error {
	// 1. 获取配额计划信息
	plan, err := m.planStorager.FetchQuotaPlan(ctx, &QuotaPlanFilter{ID: &planID})
	if err != nil {
		return fmt.Errorf("fetch plan %d error: %v", planID, err)
	}
	if plan == nil || plan.Quota == nil {
		return fmt.Errorf("plan %d not found or quota is nil", planID)
	}

	// 2. 获取该配额计划下的所有 API-Key
	apiKeys, err := m.apiKeyStorager.FetchAPIKeyList(ctx, &api_key.APIKeyFilter{
		QuotaPlanID: &planID,
	})
	if err != nil {
		return fmt.Errorf("fetch api keys for plan %d error: %v", planID, err)
	}

	// 3. 获取该配额计划下的所有 Entity
	var entities []*entity.EntityParam
	if m.entityStorager != nil {
		entities, err = m.entityStorager.FetchEntityList(ctx, &entity.EntityFilter{
			QuotaPlanID: &planID,
		})
		if err != nil {
			return fmt.Errorf("fetch entities for plan %d error: %v", planID, err)
		}
	}

	if m.quotaCache == nil {
		return nil
	}

	// 4. 遍历每个 API-Key，重置其 Redis 值为配额总量
	for _, apiKey := range apiKeys {
		if apiKey.Key == nil || apiKey.KeyCreateAt == nil {
			continue
		}

		if err := m.quotaCache.ResetToQuotaAtomic(ctx, *apiKey.Key, plan.Quota, plan.Unit); err != nil {
			fmt.Printf("Failed to reset Redis value for API-Key %s (key=%s): %v\n",
				*apiKey.ID, *apiKey.Key, err)
			continue
		}

		fmt.Printf("Reset Redis value for API-Key %s (key=%s) to quota %v\n",
			*apiKey.ID, *apiKey.Key, *plan.Quota)
	}

	// 5. 遍历每个 Entity，重置其 Redis 值为配额总量
	for _, entity := range entities {
		if entity.EntityID == nil {
			continue
		}

		if err := m.quotaCache.ResetToQuotaAtomic(ctx, *entity.EntityID, plan.Quota, plan.Unit); err != nil {
			fmt.Printf("Failed to reset Redis value for Entity %s: %v\n",
				*entity.EntityID, err)
			continue
		}

		fmt.Printf("Reset Redis value for Entity %s to quota %v\n",
			*entity.EntityID, *plan.Quota)
	}

	return nil
}
