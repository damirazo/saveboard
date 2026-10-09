export function Divider({ label = 'или' }: { label?: string }) {
  return <div className="auth-divider"><span>{label}</span></div>;
}
