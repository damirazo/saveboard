import { KeyRound } from 'lucide-react';
import { Button } from '../ui/button';

export function OAuthPlaceholder({ label = 'Продолжить через OAuth', description = 'Вход через внешний аккаунт появится позже.' }: { label?: string; description?: string }) {
  return <div className="oauth-placeholder"><Button variant="outline" disabled><KeyRound aria-hidden="true" />{label}<span>Скоро</span></Button><p>{description}</p></div>;
}
