import { Search } from 'lucide-react';
import { Input } from '../ui/form-field';

export function SearchPanel({ value, onChange, label = 'Поиск', placeholder = 'Поиск…' }: { value: string; onChange: (value: string) => void; label?: string; placeholder?: string }) {
  return <div className="library-search" role="search" aria-label={label}><Search aria-hidden="true" /><Input type="search" aria-label={label} placeholder={placeholder} value={value} onChange={event => onChange(event.target.value)} /></div>;
}
