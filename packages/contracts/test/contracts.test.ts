// Validates the contract sources themselves: every schema compiles in strict mode, every
// example embedded in the OpenAPI document matches its schema, and the documented WebSocket
// examples (docs/api/WEBSOCKET-EVENTS.md §7) match the envelope schemas.

import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import { describe, expect, it } from 'vitest';

const PKG = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const openapi = JSON.parse(readFileSync(path.join(PKG, 'src/generated/openapi.json'), 'utf8'));

function newAjv() {
  const ajv = new Ajv2020({ strict: true, allErrors: true });
  addFormats(ajv);
  ajv.addFormat('int64', { type: 'number', validate: (n: number) => Number.isSafeInteger(n) });
  return ajv;
}

// OpenAPI component schemas, re-rooted as JSON Schema $defs so they compile standalone.
const COMPONENTS_ID = 'urn:matecollab:openapi:components';
function componentsAjv() {
  const ajv = newAjv();
  const defs = JSON.parse(
    JSON.stringify(openapi.components.schemas).replaceAll('#/components/schemas/', '#/$defs/'),
  );
  ajv.addSchema({ $id: COMPONENTS_ID, $defs: defs });
  return ajv;
}
const componentRef = (name: string) => ({ $ref: `${COMPONENTS_ID}#/$defs/${name}` });

function resolveRef<T>(value: T | { $ref: string }): T {
  if (value && typeof value === 'object' && '$ref' in value) {
    const parts = value.$ref.replace(/^#\//, '').split('/');
    return parts.reduce(
      (node: Record<string, unknown>, key) => node[key] as Record<string, unknown>,
      openapi,
    ) as T;
  }
  return value as T;
}

interface MediaType {
  schema: unknown;
  examples?: Record<string, { value: unknown }>;
}

describe('OpenAPI document', () => {
  const ajv = componentsAjv();

  it('compiles every component schema in strict mode', () => {
    for (const name of Object.keys(openapi.components.schemas)) {
      expect(
        () => ajv.getSchema(`${COMPONENTS_ID}#/$defs/${name}`) ?? ajv.compile(componentRef(name)),
      ).not.toThrow();
    }
  });

  it('has examples that match their schemas', () => {
    let checked = 0;
    for (const [route, item] of Object.entries<Record<string, { responses: Record<string, unknown> }>>(
      openapi.paths,
    )) {
      for (const [method, op] of Object.entries(item)) {
        if (!op?.responses) continue;
        for (const [status, raw] of Object.entries(op.responses)) {
          const response = resolveRef<{ content?: Record<string, MediaType> }>(raw as never);
          for (const media of Object.values(response.content ?? {})) {
            const schema = JSON.parse(
              JSON.stringify(media.schema).replaceAll('#/components/schemas/', `${COMPONENTS_ID}#/$defs/`),
            );
            const validate = ajv.compile(schema);
            for (const [name, example] of Object.entries(media.examples ?? {})) {
              const ok = validate(example.value);
              expect(
                ok,
                `${method.toUpperCase()} ${route} ${status} example "${name}": ${ajv.errorsText(validate.errors)}`,
              ).toBe(true);
              checked++;
            }
          }
        }
      }
    }
    expect(checked).toBeGreaterThan(0);
  });

  it('rejects undocumented fields in runtime config', () => {
    const validate = ajv.compile(componentRef('RuntimeConfig'));
    const valid = {
      environment: 'production',
      version: '1.0.0',
      api_version: 'v1',
      public_url: 'https://collab.example.org',
      realtime: { path: '/api/v1/realtime', protocol: 'mc.v1' },
      media: { livekit_url: null },
      uploads: { max_upload_bytes: 1 },
      features: { org_signup: true },
    };
    expect(validate(valid)).toBe(true);
    expect(validate({ ...valid, secret: 'x' })).toBe(false);
  });

  it('requires problem types to use the MateCollab URN', () => {
    const validate = ajv.compile(componentRef('ProblemDetails'));
    const problem = {
      type: 'urn:matecollab:problem:not_found',
      title: 'Not Found',
      status: 404,
      code: 'not_found',
    };
    expect(validate(problem)).toBe(true);
    expect(validate({ ...problem, type: 'about:blank' })).toBe(false);
  });
});

describe('WebSocket envelope schemas', () => {
  const ajv = newAjv();
  for (const file of readdirSync(path.join(PKG, 'events')).filter((f) => f.endsWith('.schema.json'))) {
    ajv.addSchema(JSON.parse(readFileSync(path.join(PKG, 'events', file), 'utf8')));
  }
  const validator = (id: string) => {
    const v = ajv.getSchema(`urn:matecollab:events:${id}`);
    if (!v) throw new Error(`schema ${id} not found`);
    return v;
  };

  const welcome = {
    v: 1,
    id: '0199b2c0-1111-7000-8000-000000000001',
    type: 'welcome',
    ts: '2026-10-12T09:59:58.010Z',
    org_id: '0199a6f0-1d2e-7c11-9a55-0b3c4d5e6f70',
    data: { connection_id: 'c_8f2a', protocol: 'mc.v1', heartbeat_ms: 25000 },
  };
  const messageCreated = {
    v: 1,
    id: '0199b2c4-8c30-7f12-a3c1-5b0e7d9f1c22',
    type: 'message.created',
    ts: '2026-10-12T10:04:11.502Z',
    org_id: '0199a6f0-1d2e-7c11-9a55-0b3c4d5e6f70',
    conversation_id: '0199a6f2-5b1c-7e44-8a09-6c2d1e3f4a50',
    change_seq: 5187,
    data: {},
  };

  it('accepts the documented server events', () => {
    const v = validator('server-event');
    expect(v(welcome), ajv.errorsText(v.errors)).toBe(true);
    expect(v(messageCreated), ajv.errorsText(v.errors)).toBe(true);
    expect(v({ ...messageCreated, type: 'message.created.v2' })).toBe(true);
  });

  it('rejects malformed server events', () => {
    const v = validator('server-event');
    const { change_seq: _omit, ...withoutSeq } = messageCreated;
    expect(v(withoutSeq)).toBe(false); // conversation_id requires change_seq
    expect(v({ ...welcome, v: 2 })).toBe(false);
    expect(v({ ...welcome, type: 'Message Created' })).toBe(false);
    expect(v({ ...welcome, extra: true })).toBe(false);
  });

  it('accepts client commands and replies', () => {
    const command = validator('client-command');
    const reply = validator('command-reply');
    expect(command({ v: 1, id: 'c-17', type: 'subscribe', data: { topics: [] } })).toBe(true);
    expect(reply({ v: 1, type: 'ack', re: 'c-17', data: { subscribed: [] } })).toBe(true);
    expect(
      reply({
        v: 1,
        type: 'error',
        re: 'c-21',
        data: { code: 'rate_limited', message: 'Slow down.', retry_after_ms: 3000 },
      }),
    ).toBe(true);
    expect(reply({ v: 1, type: 'error', re: 'c-21', data: {} })).toBe(false); // errors carry code + message
  });
});
