package repository

import (
	"context"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// PlanRemeasureRepository 管理审批后现场复测记录，并提供“按分区取最新合格复测”
// 的派生查询，供计划列表/详情与阀门页统一读取生效含水率。
type PlanRemeasureRepository interface {
	Create(context.Context, *model.PlanRemeasure) error
	ExistsByPlanID(context.Context, uint) (bool, error)
	GetByPlanID(context.Context, uint) (model.PlanRemeasure, error)
	// LatestForZone 返回 zoneCode 分区内 measuredAt >= cutoff 的最新一条复测；
	// zoneCode 为空时只在 ownPlanID 这一条计划的复测中选取。
	LatestForZone(ctx context.Context, zoneCode string, ownPlanID uint, cutoff time.Time) (model.PlanRemeasure, error)
	// EffectiveFor 批量派生：每条业务记录按自己的 cutoff 选取合格复测。
	EffectiveFor(ctx context.Context, keys []ZoneCutoff) (map[string]model.PlanRemeasure, error)
	// RegisteredPlanIDs 返回全部已补登复测的计划 ID，供列表批量标记。
	RegisteredPlanIDs(ctx context.Context) (map[uint]bool, error)
	ListAll(context.Context) ([]model.PlanRemeasure, error)
}

// ZoneCutoff 描述一条业务记录参与派生时使用的分区键与生效时间下界。
type ZoneCutoff struct {
	Key      string
	OwnPlan  uint
	ZoneCode string
	Cutoff   time.Time
}

type planRemeasureRepository struct {
	db *gorm.DB
}

func NewPlanRemeasureRepository(db *gorm.DB) PlanRemeasureRepository {
	return &planRemeasureRepository{db: db}
}

func (r *planRemeasureRepository) Create(ctx context.Context, item *model.PlanRemeasure) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *planRemeasureRepository) ExistsByPlanID(ctx context.Context, planID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.PlanRemeasure{}).Where("plan_id = ?", planID).Count(&count).Error
	return count > 0, err
}

func (r *planRemeasureRepository) GetByPlanID(ctx context.Context, planID uint) (model.PlanRemeasure, error) {
	var item model.PlanRemeasure
	err := r.db.WithContext(ctx).Where("plan_id = ?", planID).First(&item).Error
	return item, err
}

func (r *planRemeasureRepository) LatestForZone(ctx context.Context, zoneCode string, ownPlanID uint, cutoff time.Time) (model.PlanRemeasure, error) {
	var item model.PlanRemeasure
	q := r.db.WithContext(ctx).Where("measured_at >= ?", cutoff).Order("measured_at DESC, id DESC")
	if zoneCode != "" {
		q = q.Where("zone_code = ?", zoneCode)
	} else {
		q = q.Where("plan_id = ?", ownPlanID)
	}
	err := q.First(&item).Error
	return item, err
}

// EffectiveFor 一次载入全部分区复测，在内存中按每条记录各自的生效时间下界挑
// 最新一条。复测表规模与计划数同阶，避免 N+1 查询和跨方言窗口函数。
func (r *planRemeasureRepository) EffectiveFor(ctx context.Context, keys []ZoneCutoff) (map[string]model.PlanRemeasure, error) {
	result := make(map[string]model.PlanRemeasure, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	all, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		var picked *model.PlanRemeasure
		for i := range all {
			candidate := &all[i]
			if candidate.MeasuredAt.Before(key.Cutoff) {
				continue
			}
			if key.ZoneCode != "" {
				if candidate.ZoneCode != key.ZoneCode {
					continue
				}
			} else if candidate.PlanID != key.OwnPlan {
				continue
			}
			if picked == nil || candidate.MeasuredAt.After(picked.MeasuredAt) ||
				(candidate.MeasuredAt.Equal(picked.MeasuredAt) && candidate.ID > picked.ID) {
				picked = candidate
			}
		}
		if picked != nil {
			result[key.Key] = *picked
		}
	}
	return result, nil
}

func (r *planRemeasureRepository) RegisteredPlanIDs(ctx context.Context) (map[uint]bool, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).Model(&model.PlanRemeasure{}).Pluck("plan_id", &ids).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func (r *planRemeasureRepository) ListAll(ctx context.Context) ([]model.PlanRemeasure, error) {
	items := make([]model.PlanRemeasure, 0)
	err := r.db.WithContext(ctx).Order("measured_at DESC, id DESC").Find(&items).Error
	return items, err
}
