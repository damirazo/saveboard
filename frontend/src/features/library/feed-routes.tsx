import { useParams, useSearch } from '@tanstack/react-router';
import { LibraryPage } from './library-page';

export function SearchPage() {
  const { tag, label, scope } = useSearch({ from: '/search' });
  return <LibraryPage key={tag ?? 'search'} searching initialMine={scope === 'mine'} initialTag={tag ? { id: tag, title: label || tag } : undefined} />;
}

export function ProfilePage() {
  const { username } = useParams({ from: '/profiles/$username' });
  return <LibraryPage key={username} author={username} />;
}
