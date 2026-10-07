package service

import (
	"context"
	"testing"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/config"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"gorm.io/gorm"
)

type fakeSecurity struct{}

func (fakeSecurity) Login(context.Context, dto.LoginRequest) (dto.LoginResponse, error) {
	return dto.LoginResponse{}, nil
}
func (fakeSecurity) Audit(context.Context, string, string, string, string, uint, string, string, string) error {
	return nil
}
func (fakeSecurity) ListAudits(context.Context, int, int, string) ([]model.AuditLog, int64, error) {
	return nil, 0, nil
}
func (fakeSecurity) AuditSummary(context.Context, time.Duration) (model.AuditSummary, error) {
	return model.AuditSummary{}, nil
}
func (fakeSecurity) EntityHistory(context.Context, string, uint, int) ([]model.AuditLog, error) {
	return nil, nil
}
func (fakeSecurity) RuntimeConfig() config.PublicConfig { return config.PublicConfig{} }

type fakePlanRepository struct {
	items map[uint]model.IrrigationPlan
}

func newFakePlanRepository() *fakePlanRepository {
	return &fakePlanRepository{items: make(map[uint]model.IrrigationPlan)}
}

func (r *fakePlanRepository) List(_ context.Context, _ dto.PageQuery) (repository.Page[model.IrrigationPlan], error) {
	items := make([]model.IrrigationPlan, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return repository.Page[model.IrrigationPlan]{Items: items, Total: int64(len(items)), Page: 1, PageSize: 20}, nil
}
func (r *fakePlanRepository) Get(_ context.Context, id uint) (model.IrrigationPlan, error) {
	item, ok := r.items[id]
	if !ok {
		return model.IrrigationPlan{}, gorm.ErrRecordNotFound
	}
	return item, nil
}
func (r *fakePlanRepository) Create(_ context.Context, item *model.IrrigationPlan) error {
	item.ID = uint(len(r.items) + 1)
	r.items[item.ID] = *item
	return nil
}
func (r *fakePlanRepository) Update(_ context.Context, id, _ uint, item *model.IrrigationPlan) error {
	r.items[id] = *item
	return nil
}
func (r *fakePlanRepository) Delete(_ context.Context, id uint) error {
	delete(r.items, id)
	return nil
}
func (r *fakePlanRepository) CountByStatus(context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}

type fakeRemeasureRepository struct {
	items  map[uint]model.PlanRemeasure
	nextID uint
}

func newFakeRemeasureRepository() *fakeRemeasureRepository {
	return &fakeRemeasureRepository{items: make(map[uint]model.PlanRemeasure)}
}

func (r *fakeRemeasureRepository) Create(_ context.Context, item *model.PlanRemeasure) error {
	for _, existing := range r.items {
		if existing.PlanID == item.PlanID {
			return gorm.ErrDuplicatedKey
		}
	}
	r.nextID++
	item.ID = r.nextID
	r.items[item.ID] = *item
	return nil
}
func (r *fakeRemeasureRepository) GetByPlanID(_ context.Context, planID uint) (model.PlanRemeasure, error) {
	for _, item := range r.items {
		if item.PlanID == planID {
			return item, nil
		}
	}
	return model.PlanRemeasure{}, gorm.ErrRecordNotFound
}
func (r *fakeRemeasureRepository) ByPlanIDs(_ context.Context, planIDs []uint) (map[uint]model.PlanRemeasure, error) {
	result := make(map[uint]model.PlanRemeasure, len(planIDs))
	for _, id := range planIDs {
		for _, item := range r.items {
			if item.PlanID == id {
				result[id] = item
			}
		}
	}
	return result, nil
}
func (r *fakeRemeasureRepository) LatestByFacilities(_ context.Context, facilities []string) (map[string]model.PlanRemeasure, error) {
	result := make(map[string]model.PlanRemeasure, len(facilities))
	for _, facility := range facilities {
		var newest *model.PlanRemeasure
		for _, item := range r.items {
			if item.Facility != facility {
				continue
			}
			candidate := item
			if newest == nil || candidate.MeasuredAt.After(newest.MeasuredAt) ||
				(candidate.MeasuredAt.Equal(newest.MeasuredAt) && candidate.ID > newest.ID) {
				newest = &candidate
			}
		}
		if newest != nil {
			result[facility] = *newest
		}
	}
	return result, nil
}

func seedPlanForRemeasure(t *testing.T, svc IrrigationPlanService, code, facility string, metricValue float64, effectiveAt time.Time) model.IrrigationPlan {
	t.Helper()
	moisture := metricValue
	plan, err := svc.Create(context.Background(), dto.CreateIrrigationPlan{
		Code: code, Name: "计划" + code, Facility: facility, Owner: "运行一组",
		Category: "常规", RiskLevel: "low", MetricValue: moisture, MetricUnit: "%",
		EffectiveAt: effectiveAt,
	}, "operator", "req-test")
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return plan
}

func remeasureInput(measuredAt time.Time, moisture float64) dto.RemeasureIrrigationPlan {
	return dto.RemeasureIrrigationPlan{MeasuredAt: measuredAt, Moisture: &moisture}
}

func TestRemeasureKeepsApprovedMetricAndDerivesEffective(t *testing.T) {
	svc := NewIrrigationPlanService(newFakePlanRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()
	plan := seedPlanForRemeasure(t, svc, "IP-T01", "分区A", 25.0, now)

	updated, err := svc.Remeasure(context.Background(), plan.ID, remeasureInput(now.Add(time.Hour), 31.5), "operator", "req-test")
	if err != nil {
		t.Fatalf("remeasure: %v", err)
	}
	if updated.MetricValue != 25.0 {
		t.Fatalf("approved metric must stay pinned, got %v", updated.MetricValue)
	}
	if updated.EffectiveMetricValue != 31.5 || updated.EffectiveMetricSource != "remeasure" {
		t.Fatalf("effective metric should come from remeasure, got %v (%s)", updated.EffectiveMetricValue, updated.EffectiveMetricSource)
	}
	if updated.RemeasureMoisture == nil || *updated.RemeasureMoisture != 31.5 || updated.RemeasuredAt == nil {
		t.Fatalf("plan's own remeasure should be exposed, got %+v", updated.RemeasureMoisture)
	}
	if updated.EffectiveMeasuredAt == nil || !updated.EffectiveMeasuredAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("effective measured time mismatch: %v", updated.EffectiveMeasuredAt)
	}
}

func TestRemeasureBeforeEffectiveTimeRejected(t *testing.T) {
	svc := NewIrrigationPlanService(newFakePlanRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()
	plan := seedPlanForRemeasure(t, svc, "IP-T02", "分区B", 25.0, now)

	_, err := svc.Remeasure(context.Background(), plan.ID, remeasureInput(now.Add(-time.Minute), 31.5), "operator", "req-test")
	if err != ErrRemeasureBeforeEffective {
		t.Fatalf("expected ErrRemeasureBeforeEffective, got %v", err)
	}
}

func TestRemeasureOnlyOncePerPlan(t *testing.T) {
	svc := NewIrrigationPlanService(newFakePlanRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()
	plan := seedPlanForRemeasure(t, svc, "IP-T03", "分区C", 25.0, now)

	if _, err := svc.Remeasure(context.Background(), plan.ID, remeasureInput(now.Add(time.Hour), 31.5), "operator", "req-test"); err != nil {
		t.Fatalf("first remeasure: %v", err)
	}
	_, err := svc.Remeasure(context.Background(), plan.ID, remeasureInput(now.Add(2*time.Hour), 40.0), "operator", "req-test")
	if err != ErrRemeasureExists {
		t.Fatalf("expected ErrRemeasureExists, got %v", err)
	}
}

func TestEffectiveMetricUsesNewestRemeasurePerFacility(t *testing.T) {
	svc := NewIrrigationPlanService(newFakePlanRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()
	first := seedPlanForRemeasure(t, svc, "IP-T04", "分区D", 25.0, now)
	second := seedPlanForRemeasure(t, svc, "IP-T05", "分区D", 26.0, now)

	if _, err := svc.Remeasure(context.Background(), first.ID, remeasureInput(now.Add(time.Hour), 30.0), "operator", "req-test"); err != nil {
		t.Fatalf("remeasure first plan: %v", err)
	}
	if _, err := svc.Remeasure(context.Background(), second.ID, remeasureInput(now.Add(2*time.Hour), 36.5), "operator", "req-test"); err != nil {
		t.Fatalf("remeasure second plan: %v", err)
	}

	got, err := svc.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("get first plan: %v", err)
	}
	if got.EffectiveMetricValue != 36.5 || got.EffectiveMetricSource != "remeasure" {
		t.Fatalf("newest remeasure of the zone should win, got %v (%s)", got.EffectiveMetricValue, got.EffectiveMetricSource)
	}
	if got.RemeasureMoisture == nil || *got.RemeasureMoisture != 30.0 {
		t.Fatalf("plan's own remeasure should stay 30.0, got %+v", got.RemeasureMoisture)
	}

	page, err := svc.List(context.Background(), dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	for _, item := range page.Items {
		if item.EffectiveMetricValue != 36.5 {
			t.Fatalf("list should show the zone-newest remeasure on every plan, got %v for %s", item.EffectiveMetricValue, item.Code)
		}
	}
}

func TestEffectiveMetricFallsBackToApprovedValue(t *testing.T) {
	svc := NewIrrigationPlanService(newFakePlanRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()
	plan := seedPlanForRemeasure(t, svc, "IP-T06", "分区E", 25.0, now)

	got, err := svc.Get(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if got.EffectiveMetricValue != 25.0 || got.EffectiveMetricSource != "approved" || got.EffectiveMeasuredAt != nil {
		t.Fatalf("without remeasure the approved metric stays effective, got %v (%s)", got.EffectiveMetricValue, got.EffectiveMetricSource)
	}
}
