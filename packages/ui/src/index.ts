// @matecollab/ui — design system entry point. Styles: import '@matecollab/ui/theme.css' after
// Tailwind in the consuming stylesheet.

export { cn } from './lib/cn';

export { Button, buttonVariants, type ButtonProps } from './components/button';
export { IconButton, iconButtonVariants, type IconButtonProps } from './components/icon-button';
export { Tooltip, TooltipProvider, type TooltipProps } from './components/tooltip';
export { Field, useFieldControl, type FieldProps } from './components/field';
export { Input, Textarea, controlClasses } from './components/input';
export { Select, type SelectOption, type SelectProps } from './components/select';
export {
  Checkbox,
  ChoiceLabel,
  RadioCard,
  RadioCardGroup,
  RadioGroup,
  RadioGroupItem,
  Switch,
  type RadioCardProps,
} from './components/choice';
export {
  Avatar,
  AvatarGroup,
  PresenceDot,
  initialsOf,
  presenceLabels,
  toneOf,
  type AvatarProps,
  type Presence,
} from './components/avatar';
export { Badge, Kbd, NavCount, badgeVariants, type BadgeProps } from './components/badge';
export {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from './components/menu';
export { Tabs, TabsContent, TabsList, TabsTrigger } from './components/tabs';
export {
  ConfirmDialog,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTrigger,
  type ConfirmDialogProps,
  type DialogContentProps,
} from './components/dialog';
export { Drawer, type DrawerProps } from './components/drawer';
export { Toaster, dismissToast, toast, type ToastOptions } from './components/toast';
export {
  EmptyState,
  Progress,
  Skeleton,
  Spinner,
  type EmptyStateProps,
  type ProgressProps,
} from './components/feedback';
export { SearchTrigger, useModifierKeyLabel, type SearchTriggerProps } from './components/search-trigger';
export { ResizeHandle, type ResizeHandleProps } from './components/resize-handle';
export { BRAND_BASE_PATH, BrandLockup, BrandMark, brandFiles } from './components/brand';
export { VisuallyHidden } from 'radix-ui';

export {
  AppFrame,
  SIDEBAR_DEFAULT_WIDTH,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_MIN_WIDTH,
  useShell,
  type AppFrameProps,
} from './shell/app-frame';
export { Rail, RailItem, type RailItemProps } from './shell/rail';
export { Breadcrumb, GlobalBar } from './shell/global-bar';
export {
  OrgBadge,
  OrgSwitcherButton,
  Sidebar,
  SidebarItem,
  SidebarSectionLabel,
  SidebarTitle,
  type SidebarItemProps,
} from './shell/sidebar';
export { ContextPanel, type ContextPanelProps } from './shell/context-panel';
export { Page, PageHeader, SectionTitle } from './shell/page';

export {
  StatePattern,
  statePatterns,
  type StatePatternDefinition,
  type StatePatternId,
} from './patterns/state-patterns';
