import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { Field } from '../components/field';
import { Select } from '../components/select';
import { StatePattern, statePatterns, type StatePatternId } from '../patterns/state-patterns';
import { PageHeader } from '../shell/page';

// The prototype's "Interaction review" page: every empty, loading and recovery state.
const meta: Meta<typeof StatePattern> = {
  title: 'Patterns/States',
  component: StatePattern,
  decorators: [
    (Story, context) =>
      context.parameters.stage === false ? (
        <Story />
      ) : (
        <div className="flex min-h-82.5 items-center justify-center rounded-md border border-border bg-surface">
          <Story />
        </div>
      ),
  ],
};
export default meta;
type Story = StoryObj<typeof StatePattern>;

const state = (id: StatePatternId): Story => ({ args: { id } });

export const EmptyChat = state('empty-chat');
export const NoSpaces = state('no-spaces');
export const NoMeetings = state('no-meetings');
export const NoFiles = state('no-files');
export const NoSearchResults = state('no-search-results');
export const LoadingMessages = state('loading-messages');
export const UploadingFile = state('uploading-file');
export const UploadFailure = state('upload-failure');
export const Disconnected = state('disconnected');
export const CallConnectionIssue = state('call-connection-issue');
export const PermissionDenied = state('permission-denied');
export const IntegrationError = state('integration-error');
export const ExpiredInvitation = state('expired-invitation');
export const GuestRestricted = state('guest-restricted');
export const Offline = state('offline');

export const InteractionReview: Story = {
  parameters: { stage: false },
  tags: ['novisual'],
  render: function Render() {
    const [id, setId] = useState<StatePatternId>('empty-chat');
    return (
      <div>
        <PageHeader
          eyebrow="Interaction review"
          title="The moments in between"
          description="Empty, loading and recovery states, without changing any workspace data."
        />
        <div className="max-w-xs">
          <Field label="Preview state">
            <Select
              value={id}
              onValueChange={(value) => setId(value as StatePatternId)}
              options={statePatterns.map((p) => ({ value: p.id, label: p.name }))}
            />
          </Field>
        </div>
        <div className="flex min-h-82.5 items-center justify-center rounded-md border border-border bg-surface">
          <StatePattern id={id} />
        </div>
      </div>
    );
  },
};
