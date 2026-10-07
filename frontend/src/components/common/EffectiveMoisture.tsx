import { Tag, Tooltip } from 'antd';
import type { DomainRecord } from '../../types/domain';
import { formatDate } from '../../utils/format';

function resolveValue(record: DomainRecord): { value: number; from: string } {
  if (typeof record.effectiveMoisture === 'number') {
    return { value: record.effectiveMoisture, from: record.effectiveMoistureFrom || 'approved' };
  }
  return { value: record.metricValue, from: 'approved' };
}

function approvedBaseline(record: DomainRecord): number {
  return typeof record.approvedMetricValue === 'number' ? record.approvedMetricValue : record.metricValue;
}

// EffectiveMoisture 统一展示页面应执行的含水率：复测生效时标记“复测”，否则标记
// “审批”。审批指标本身始终保留在 record.metricValue 上。
export function EffectiveMoisture({ record }: { record: DomainRecord }) {
  const { value, from } = resolveValue(record);
  const fromRemeasure = from === 'remeasure';
  const content = <span className="effective-moisture">
    <strong>{value.toFixed(1)}%</strong>
    {fromRemeasure
      ? <Tag color="processing" className="effective-moisture__tag">复测</Tag>
      : <Tag className="effective-moisture__tag">审批</Tag>}
  </span>;
  return fromRemeasure && record.remeasuredAt
    ? <Tooltip title={`复测时间：${formatDate(record.remeasuredAt)}；审批指标：${approvedBaseline(record)}%`}>{content}</Tooltip>
    : <Tooltip title={`审批指标：${approvedBaseline(record)}%`}>{content}</Tooltip>;
}
