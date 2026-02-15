export default function StatusDot({ online, muted }: { online: boolean; muted?: boolean }) {
  if (muted) {
    return <span className="inline-block w-2.5 h-2.5 rounded-full bg-gray-400" title="Alerts muted" />;
  }
  return (
    <span
      className={`inline-block w-2.5 h-2.5 rounded-full ${online ? 'bg-green-500' : 'bg-red-500'}`}
      title={online ? 'Online' : 'Offline'}
    />
  );
}
