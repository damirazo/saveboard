import createClient from 'openapi-fetch';
import createQueryClient from 'openapi-react-query';
import type { paths } from './schema';
import { createSessionClient } from './session';

export type { paths, components } from './schema';
export const sessionClient = createSessionClient(window.sessionStorage, (...args) => fetch(...args));
export const apiClient = createClient<paths>({ baseUrl: window.location.origin, credentials: 'same-origin', fetch: sessionClient.fetch });
export const api = createQueryClient(apiClient);
