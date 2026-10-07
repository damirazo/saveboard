import createClient from 'openapi-fetch';
import createQueryClient from 'openapi-react-query';
import type { paths } from './schema';

export type { paths, components } from './schema';
export const apiClient = createClient<paths>({ baseUrl: '', credentials: 'same-origin' });
export const api = createQueryClient(apiClient);
