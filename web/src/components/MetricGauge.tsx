interface MetricGaugeProps {
  label: string;
  value: number;
  warnAt?: number;
  critAt?: number;
  unit?: string;
  size?: 'sm' | 'lg';
}

export default function MetricGauge({ label, value, warnAt = 80, critAt = 95, unit = '%', size = 'sm' }: MetricGaugeProps) {
  const color = value >= critAt ? 'text-red-600' : value >= warnAt ? 'text-amber-500' : 'text-green-600';
  const bgColor = value >= critAt ? 'bg-red-500' : value >= warnAt ? 'bg-amber-400' : 'bg-green-500';

  if (size === 'lg') {
    return (
      <div className="bg-white rounded-lg border p-4">
        <div className="text-sm text-gray-500 mb-1">{label}</div>
        <div className={`text-3xl font-bold ${color}`}>{value.toFixed(1)}{unit}</div>
        <div className="mt-2 h-2 bg-gray-100 rounded-full overflow-hidden">
          <div className={`h-full ${bgColor} rounded-full transition-all`} style={{ width: `${Math.min(value, 100)}%` }} />
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="text-xs text-gray-500">{label}</div>
      <div className={`text-sm font-semibold ${color}`}>{value.toFixed(1)}{unit}</div>
      <div className="mt-0.5 h-1 bg-gray-100 rounded-full overflow-hidden w-16">
        <div className={`h-full ${bgColor} rounded-full`} style={{ width: `${Math.min(value, 100)}%` }} />
      </div>
    </div>
  );
}
