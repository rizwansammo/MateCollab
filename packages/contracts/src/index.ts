// Public entry point of @matecollab/contracts. Types come from generated code; never hand-write
// an API shape in the web app — add it to openapi/ or events/ and run `pnpm gen`.

import type { components } from './generated/openapi';

export type { components, operations, paths } from './generated/openapi';
export type { ServerEvent, ClientCommand, CommandReply } from './generated/events';

export type RuntimeConfig = components['schemas']['RuntimeConfig'];
export type ProblemDetails = components['schemas']['ProblemDetails'];
export type FieldError = components['schemas']['FieldError'];
export type PageInfo = components['schemas']['PageInfo'];

/** Base path of the REST API (same origin as the web app). */
export const API_BASE_PATH = '/api/v1';

/** Prefix of every problem `type`; the suffix equals `code`. */
export const PROBLEM_TYPE_PREFIX = 'urn:matecollab:problem:';

/** Media type of error responses. */
export const PROBLEM_CONTENT_TYPE = 'application/problem+json';

/** WebSocket subprotocol this client speaks. */
export const REALTIME_PROTOCOL = 'mc.v1';
