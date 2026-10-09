import { apiClient, sessionClient, type components } from '@saveboard/api-client';

export type SavedPost = Omit<components['schemas']['View'], 'tags' | 'photos'> & { tags: PostTag[]; photos: Photo[] };
export type PostWrite = Omit<components['schemas']['Write'], 'tags' | 'photo_ids'> & { tags: NonNullable<components['schemas']['Write']['tags']>; photo_ids: string[] };
export type PostTag = components['schemas']['Tag'];
export type Photo = components['schemas']['PhotoView'];
export function apiError(error: unknown): Error {
  if (error instanceof Error) return error;
  const value = error as { detail?: string; errors?: { message?: string }[] };
  return new Error(value?.errors?.map(e => e.message).join('; ') || value?.detail || 'Не удалось выполнить запрос. Попробуйте ещё раз.');
}
export async function savePost(body: PostWrite, id?: string) {
  const result = id ? await apiClient.PUT('/api/v1/posts/{id}', { params: { path: { id } }, body }) : await apiClient.POST('/api/v1/posts', { body });
  if (result.error || !result.data) throw apiError(result.error);
  return result.data;
}
export async function photoBlob(photo: Photo, signal?: AbortSignal) {
  const response = await sessionClient.fetch(new Request(new URL(photo.url, window.location.origin), { signal }));
  if (!response.ok) throw new Error('Фото недоступно');
  return response.blob();
}
export async function uploadPhoto(file: File) {
  const data = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader(); reader.onerror = reject;
    reader.onload = () => resolve(String(reader.result).split(',')[1] ?? ''); reader.readAsDataURL(file);
  });
  const result = await apiClient.POST('/api/v1/photos', { body: { data } });
  if (result.error || !result.data) throw apiError(result.error);
  return result.data;
}
