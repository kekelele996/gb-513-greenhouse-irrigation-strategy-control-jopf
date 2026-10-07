import { Tag } from 'antd';
import type { DomainRecord } from '../../types/domain';

// EffectiveMetric 统一展示生效指标：分区有复测时显示复测后的数并打标，
// 否则回落到审批值；计划列表与阀门执行列表共用。
export function EffectiveMetric({ record }: { record: DomainRecord }) {
  const value = record.effectiveMetricValue ?? record.metricValue;
  const unit = record.effectiveMetricUnit || record.metricUnit;
  const remeasured = record.effectiveMetricSource === 'remeasure';
  return <span className="effective-metric">
    <strong>{value} {unit}</strong>
    {remeasured && <Tag color="cyan">复测</Tag>}
  </span>;
}
