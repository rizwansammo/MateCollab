import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { Monitor, Moon, Sun } from 'lucide-react';
import {
  Checkbox,
  ChoiceLabel,
  RadioCard,
  RadioCardGroup,
  RadioGroup,
  RadioGroupItem,
  Switch,
} from '../components/choice';
import { Field } from '../components/field';
import { Input, Textarea } from '../components/input';
import { Select } from '../components/select';

const meta: Meta = { title: 'Components/Forms' };
export default meta;
type Story = StoryObj;

export const TextFields: Story = {
  render: () => (
    <div className="grid max-w-xl gap-x-4.5 phone:grid-cols-2">
      <Field label="Space name" required description="Visible to everyone in the organisation.">
        <Input defaultValue="Engineering" />
      </Field>
      <Field label="Work email" error="Enter an email address like name@company.com.">
        <Input type="email" defaultValue="ayesha@" />
      </Field>
      <Field label="Search members">
        <Input placeholder="Type a name" />
      </Field>
      <Field label="Disabled">
        <Input disabled defaultValue="Managed by your administrator" />
      </Field>
      <Field label="Description" className="phone:col-span-2">
        <Textarea placeholder="What is this Space for?" />
      </Field>
    </div>
  ),
};

export const SelectField: Story = {
  render: function Render() {
    const [value, setValue] = useState('member');
    return (
      <div className="max-w-xs">
        <Field label="Role" description="Admins can manage members and settings.">
          <Select
            value={value}
            onValueChange={setValue}
            options={[
              { value: 'owner', label: 'Owner', disabled: true },
              { value: 'admin', label: 'Admin' },
              { value: 'member', label: 'Member' },
              { value: 'guest', label: 'Guest' },
            ]}
          />
        </Field>
        <Field label="Time zone">
          <Select
            placeholder="Choose a time zone"
            options={[{ value: 'Asia/Dhaka', label: 'Asia/Dhaka (UTC+06:00)' }]}
          />
        </Field>
      </div>
    );
  },
};

export const Choices: Story = {
  render: () => (
    <div className="flex max-w-md flex-col gap-6">
      <div className="flex flex-col gap-3">
        <ChoiceLabel control={<Checkbox defaultChecked />} label="Email me about mentions" />
        <ChoiceLabel
          control={<Checkbox />}
          label="Include message previews"
          description="Off by default for privacy."
        />
        <ChoiceLabel control={<Checkbox checked="indeterminate" />} label="Some channels selected" />
        <ChoiceLabel control={<Checkbox disabled />} label="Disabled option" />
      </div>
      <RadioGroup defaultValue="system" aria-label="Theme">
        <ChoiceLabel control={<RadioGroupItem value="light" />} label="Light" />
        <ChoiceLabel control={<RadioGroupItem value="dark" />} label="Dark" />
        <ChoiceLabel control={<RadioGroupItem value="system" />} label="Match system" />
      </RadioGroup>
      <div className="flex flex-col gap-3">
        <ChoiceLabel control={<Switch defaultChecked />} label="Desktop notifications" />
        <ChoiceLabel control={<Switch />} label="Reduce motion" />
        <ChoiceLabel control={<Switch disabled />} label="Disabled switch" />
      </div>
    </div>
  ),
};

function Pane({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <span data-theme={theme} className="flex h-25 flex-1 bg-bg">
      <span className="w-1/4 bg-sidebar" />
      <span className="flex flex-1 flex-col gap-1.75 p-3.5">
        <span className="h-1.25 w-full rounded-full bg-border-control/45" />
        <span className="h-1.25 w-3/5 rounded-full bg-border-control/45" />
      </span>
    </span>
  );
}

export const RadioCards: Story = {
  render: () => (
    <RadioCardGroup defaultValue="light" aria-label="Theme" className="max-w-177.5 grid-cols-3">
      <RadioCard
        value="light"
        preview={<Pane theme="light" />}
        label={
          <>
            <Sun aria-hidden /> Light
          </>
        }
      />
      <RadioCard
        value="dark"
        preview={<Pane theme="dark" />}
        label={
          <>
            <Moon aria-hidden /> Dark
          </>
        }
      />
      <RadioCard
        value="system"
        preview={
          <span className="flex">
            <Pane theme="light" />
            <Pane theme="dark" />
          </span>
        }
        label={
          <>
            <Monitor aria-hidden /> Match system
          </>
        }
      />
    </RadioCardGroup>
  ),
};
