'use client';

import { useRef, type KeyboardEvent, type PointerEvent } from 'react';
import { cn } from '../lib/cn';

export interface ResizeHandleProps {
  /** Current width in px. */
  value: number;
  min: number;
  max: number;
  /** Keyboard step in px (← / →). */
  step?: number;
  /** Called continuously while dragging or on each key press. */
  onChange: (value: number) => void;
  /** Called once a drag ends or a key press settles; persist here. */
  onCommit?: (value: number) => void;
  label: string;
  className?: string;
}

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value));

/** Vertical splitter: drag, or focus and use ←/→ (±step), Home/End (min/max). */
export function ResizeHandle({
  value,
  min,
  max,
  step = 10,
  onChange,
  onCommit,
  label,
  className,
}: ResizeHandleProps) {
  const drag = useRef<{ startX: number; startValue: number; last: number } | null>(null);

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const next =
      event.key === 'ArrowLeft'
        ? value - step
        : event.key === 'ArrowRight'
          ? value + step
          : event.key === 'Home'
            ? min
            : event.key === 'End'
              ? max
              : null;
    if (next === null) return;
    event.preventDefault();
    const clamped = clamp(next, min, max);
    onChange(clamped);
    onCommit?.(clamped);
  };

  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = { startX: event.clientX, startValue: value, last: value };
  };
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return;
    const next = clamp(drag.current.startValue + event.clientX - drag.current.startX, min, max);
    drag.current.last = next;
    onChange(next);
  };
  const onPointerUp = () => {
    if (!drag.current) return;
    onCommit?.(drag.current.last);
    drag.current = null;
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onKeyDown={onKeyDown}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      className={cn(
        'absolute top-0 -right-0.75 bottom-0 z-25 w-1.25 cursor-col-resize touch-none transition-colors duration-140',
        'hover:bg-primary/35 focus-visible:bg-primary/35 focus-visible:outline-offset-0',
        className,
      )}
    />
  );
}
