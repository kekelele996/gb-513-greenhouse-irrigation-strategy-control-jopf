import { useState } from 'react';
import { EntityPage } from '../components/EntityPage';
import { PlanDrawer } from '../components/PlanDrawer';
import { EffectiveMetric } from '../components/common/EffectiveMetric';
import { ENTITY_CONFIGS } from '../types/status';
import { useIrrigationPlanStore } from '../stores/irrigation-plan';
import { useAuth } from '../hooks/useAuth';
import { remeasureIrrigationPlan } from '../api/irrigation-plan';
import type { DomainRecord } from '../types/domain';

export default function IrrigationPlanPage() {
  const [selected, setSelected] = useState<DomainRecord | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { session } = useAuth();

  const remeasure = async (record: DomainRecord, input: { measuredAt: string; moisture: number }) => {
    setBusy(true);
    setError('');
    try {
      const result = await remeasureIrrigationPlan(record.id, input);
      setSelected(result.data);
      await useIrrigationPlanStore.getState().load('plans');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(false);
    }
  };

  return <>
    <EntityPage config={ENTITY_CONFIGS[2]} useStore={useIrrigationPlanStore} renderMetric={(item) => <EffectiveMetric record={item} />} onInspect={(item) => { setSelected(item); setError(''); }} inspectLabel="查看策略" transitionRoles={['reviewer', 'admin']} />
    <PlanDrawer open={Boolean(selected)} record={selected} kind="plan" username={session?.username || ''} role={session?.role || 'viewer'} busy={busy} error={error} onClose={() => setSelected(null)} onRemeasure={remeasure} />
  </>;
}
