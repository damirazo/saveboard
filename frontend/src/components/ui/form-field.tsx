import { useId, useState, type ComponentProps, type ReactNode } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { Button } from './button';
import { cn } from '../../lib/utils';

export function Input({ className, ...props }: ComponentProps<'input'>) {
  return <input className={cn('ui-input', className)} {...props} />;
}

export type FormFieldProps = ComponentProps<'input'> & { label: string; hint?: ReactNode; error?: string; password?: boolean };
export function FormField({ label, hint, error, password = false, id: suppliedId, 'aria-describedby': describedBy, ...props }: FormFieldProps) {
  const generatedId = useId();
  const id = suppliedId ?? generatedId;
  const [visible, setVisible] = useState(false);
  const descriptions = [describedBy, hint ? `${id}-hint` : undefined, error ? `${id}-error` : undefined].filter(Boolean).join(' ') || undefined;
  const input = <Input {...props} id={id} type={password ? (visible ? 'text' : 'password') : props.type} aria-invalid={error ? true : props['aria-invalid']} aria-describedby={descriptions} />;
  return <div className="form-field"><label htmlFor={id}>{label}</label>{password ? <div className="password-field">{input}<Button variant="icon" aria-label={visible ? 'Скрыть пароль' : 'Показать пароль'} aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? <EyeOff /> : <Eye />}</Button></div> : input}{hint && <small id={`${id}-hint`}>{hint}</small>}{error && <small id={`${id}-error`} className="field-error">{error}</small>}</div>;
}

export function FormMessage({ children, tone = 'info' }: { children: ReactNode; tone?: 'info' | 'error' }) {
  return <p className={tone === 'error' ? 'form-error' : 'form-message'} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}

export type SelectOption = { value: string; label: string };
export function Select({ label, options, className, ...props }: Omit<ComponentProps<'select'>, 'children'> & { label: string; options: readonly SelectOption[] }) {
  return <select {...props} className={cn('ui-select', className)} aria-label={label}>{options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select>;
}
