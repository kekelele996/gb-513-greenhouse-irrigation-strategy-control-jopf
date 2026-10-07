package repository

import (
	"context"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// PlanRemeasureRepository owns all persistence operations for 复测记录.
type PlanRemeasureRepository interface {
	Create(context.Context, *model.PlanRemeasure) error
	GetByPlanID(context.Context, uint) (model.PlanRemeasure, error)
	ByPlanIDs(context.Context, []uint) (map[uint]model.PlanRemeasure, error)
	LatestByFacilities(context.Context, []string) (map[string]model.PlanRemeasure, error)
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

func (r *planRemeasureRepository) GetByPlanID(ctx context.Context, planID uint) (model.PlanRemeasure, error) {
	var item model.PlanRemeasure
	err := r.db.WithContext(ctx).Where("plan_id = ?", planID).First(&item).Error
	return item, err
}

func (r *planRemeasureRepository) ByPlanIDs(ctx context.Context, planIDs []uint) (map[uint]model.PlanRemeasure, error) {
	result := make(map[uint]model.PlanRemeasure, len(planIDs))
	if len(planIDs) == 0 {
		return result, nil
	}
	items := make([]model.PlanRemeasure, 0, len(planIDs))
	if err := r.db.WithContext(ctx).Where("plan_id IN ?", planIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.PlanID] = item
	}
	return result, nil
}

// LatestByFacilities resolves 同一分区重复复测按时间取新的一条: rows come back
// newest measured time first, so the first row seen per facility wins.
func (r *planRemeasureRepository) LatestByFacilities(ctx context.Context, facilities []string) (map[string]model.PlanRemeasure, error) {
	result := make(map[string]model.PlanRemeasure, len(facilities))
	if len(facilities) == 0 {
		return result, nil
	}
	items := make([]model.PlanRemeasure, 0, len(facilities))
	if err := r.db.WithContext(ctx).
		Where("facility IN ?", facilities).
		Order("measured_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, exists := result[item.Facility]; !exists {
			result[item.Facility] = item
		}
	}
	return result, nil
}
