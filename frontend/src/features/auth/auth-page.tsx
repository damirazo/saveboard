import { useRef, useState, type FormEvent } from 'react';
import { Link } from '@tanstack/react-router';
import { ArrowRight } from 'lucide-react';
import { AppShell } from '../../components/layout/app-shell';
import { AuthLayout, FormCard } from '../../components/layout/auth-layout';
import { Button } from '../../components/ui/button';
import { FormField, FormMessage } from '../../components/ui/form-field';
import { Divider } from '../../components/ui/divider';
import { OAuthPlaceholder } from '../../components/auth/oauth-placeholder';

export type AuthValues = { username: string; email: string; login: string; password: string };
type AuthPageProps = { mode: 'login' | 'register'; onSubmit: (values: AuthValues) => Promise<void> };

export function AuthPage({ mode, onSubmit }: AuthPageProps) {
  const register = mode === 'register';
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const [error, setError] = useState('');
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (submitting.current) return; setError('');
    const data = new FormData(event.currentTarget);
    const values: AuthValues = { username: String(data.get('username') ?? '').trim(), email: String(data.get('email') ?? '').trim(), login: String(data.get('login') ?? '').trim(), password: String(data.get('password') ?? '') };
    if (!register && !values.login) { setError('Укажите username или email.'); return; }
    if (register && (Array.from(values.password).length < 8 || new TextEncoder().encode(values.password).length > 72)) { setError('Пароль должен содержать минимум 8 символов и не более 72 байт.'); return; }
    if (register && values.password !== String(data.get('passwordConfirm') ?? '')) { setError('Пароли не совпадают.'); return; }
    submitting.current = true; setPending(true);
    try { await onSubmit(values); } catch (e) { setError(e instanceof Error ? e.message : 'Не удалось выполнить запрос. Попробуйте снова.'); } finally { submitting.current = false; setPending(false); }
  }
  return <AppShell><AuthLayout title={register ? 'Всё важное —\nв одном месте.' : 'Рады видеть\nвас снова.'} description="Сохраняйте страницы, записывайте мысли и возвращайтесь к нужному в подходящий момент.">
    <FormCard title={register ? 'Создать аккаунт' : 'Вход в Saveboard'} description={register ? 'Начните собирать свою библиотеку.' : 'Введите username или email и пароль.'} footer={<p className="auth-switch">{register ? 'Уже есть аккаунт? ' : 'Нет аккаунта? '}<Link to={register ? '/login' : '/register'}>{register ? 'Войти' : 'Зарегистрироваться'}</Link></p>}>
      <form onSubmit={event => { void submit(event); }} aria-label={register ? 'Регистрация' : 'Вход'} aria-busy={pending}>
        <fieldset disabled={pending} className="auth-fields">
        {register ? <><FormField label="Username" id="username" name="username" autoComplete="username" autoCapitalize="none" spellCheck={false} pattern="[A-Za-z0-9_][A-Za-z0-9_.\-]{2,63}" minLength={3} maxLength={64} required hint="3–64 символа: латиница, цифры, _, . и -. Первый — буква, цифра или _." placeholder="например, alex" /><FormField label="Email" id="email" name="email" type="email" autoComplete="email" autoCapitalize="none" required placeholder="you@example.com" /></> : <FormField label="Username или email" id="login" name="login" autoComplete="username" autoCapitalize="none" spellCheck={false} required placeholder="alex или you@example.com" />}
        <FormField password label="Пароль" id="password" name="password" autoComplete={register ? 'new-password' : 'current-password'} required minLength={register ? 8 : undefined} hint={register ? 'От 8 символов, не более 72 байт.' : undefined} placeholder={register ? 'Придумайте надёжный пароль' : 'Введите пароль'} />
        {register && <FormField password label="Повторите пароль" id="passwordConfirm" name="passwordConfirm" autoComplete="new-password" required placeholder="Введите пароль ещё раз" />}
        </fieldset>
        {error && <FormMessage tone="error">{error}</FormMessage>}
        <Button className="auth-submit" type="submit" disabled={pending}>{pending ? 'Подождите…' : register ? 'Зарегистрироваться' : 'Войти'}<ArrowRight aria-hidden="true" /></Button>
      </form><Divider /><OAuthPlaceholder />
    </FormCard>
  </AuthLayout></AppShell>;
}
