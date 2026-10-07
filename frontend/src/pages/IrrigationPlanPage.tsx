import { useState } from 'react';
import { EntityPage } from '../components/EntityPage';
import { PlanDrawer } from '../components/PlanDrawer';
import { ENTITY_CONFIGS } from '../types/status';
import { useIrrigationPlanStore } from '../stores/irrigation-plan';
import { useAuth } from '../hooks/useAuth';
import { EffectiveMoisture } from '../components/common/EffectiveMoisture';
import type { DomainRecord } from '../types/domain';

export default function IrrigationPlanPage() {
  const [selected, setSelected] = useState<DomainRecord | null>(null);
  const { session } = useAuth();
  const reload = useIrrigationPlanStore((state) => state.load);
  return <>
    <EntityPage config={ENTITY_CONFIGS[2]} useStore={useIrrigationPlanStore} onInspect={setSelected} inspectLabel="查看策略" transitionRoles={['reviewer', 'admin']} renderMetric={(item) => <EffectiveMoisture record={item} />} />
    <PlanDrawer open={Boolean(selected)} record={selected} kind="plan" username={session?.username || ''} role={session?.role || 'viewer'} onClose={() => setSelected(null)} onRemeasured={(record) => { setSelected(record); void reload('plans'); }} />
  </>;
}
