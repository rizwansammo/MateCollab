// Public entry point of @matecollab/contracts. Types come from generated code; never hand-write
// an API shape in the web app — add it to openapi/ or events/ and run `pnpm gen`.

import type { components } from './generated/openapi';

export type { components, operations, paths } from './generated/openapi';
export type { ServerEvent, ClientCommand, CommandReply } from './generated/events';

export type RuntimeConfig = components['schemas']['RuntimeConfig'];
export type ProblemDetails = components['schemas']['ProblemDetails'];
export type FieldError = components['schemas']['FieldError'];
export type PageInfo = components['schemas']['PageInfo'];

type Schemas = components['schemas'];
export type Role = Schemas['Role'];
export type InvitableRole = Schemas['InvitableRole'];
export type MembershipStatus = Schemas['MembershipStatus'];
export type InvitationStatus = Schemas['InvitationStatus'];
export type User = Schemas['User'];
export type SessionDocument = Schemas['SessionDocument'];
export type OrganisationSummary = Schemas['OrganisationSummary'];
export type Organisation = Schemas['Organisation'];
export type SecuritySettings = Schemas['SecuritySettings'];
export type SecuritySettingsDocument = Schemas['SecuritySettingsDocument'];
export type Member = Schemas['Member'];
export type Invitation = Schemas['Invitation'];
export type Space = Schemas['Space'];
export type SpaceInput = Schemas['SpaceInput'];
export type SpaceMember = Schemas['SpaceMember'];
export type Channel = Schemas['Channel'];
export type ChannelInput = Schemas['ChannelInput'];
export type ChannelMember = Schemas['ChannelMember'];
export type MessagingSettings = Schemas['MessagingSettings'];
export type MessagingSettingsDocument = Schemas['MessagingSettingsDocument'];
export type InvitationPreview = Schemas['InvitationPreview'];
export type SessionItem = Schemas['SessionItem'];
export type EmailAddress = Schemas['EmailAddress'];
export type MFAStatus = Schemas['MFAStatus'];
export type TOTPSetup = Schemas['TOTPSetup'];

/** Base path of the REST API (same origin as the web app). */
export const API_BASE_PATH = '/api/v1';

/** Prefix of every problem `type`; the suffix equals `code`. */
export const PROBLEM_TYPE_PREFIX = 'urn:matecollab:problem:';

/** Media type of error responses. */
export const PROBLEM_CONTENT_TYPE = 'application/problem+json';

/** WebSocket subprotocol this client speaks. */
export const REALTIME_PROTOCOL = 'mc.v1';
