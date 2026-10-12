import '@testing-library/jest-dom/vitest';

// jsdom lacks a few browser APIs that Radix primitives and the shell use.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= ResizeObserverStub as unknown as typeof ResizeObserver;

let drawerViewport = false;
/** Tests switch the simulated viewport between desktop and the ≤ 1100 px drawer layout. */
export function setDrawerViewport(value: boolean) {
  drawerViewport = value;
}
window.matchMedia ??= (query: string) =>
  ({
    get matches() {
      return query.includes('max-width: 1100px') ? drawerViewport : false;
    },
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }) as unknown as MediaQueryList;

Element.prototype.hasPointerCapture ??= () => false;
Element.prototype.setPointerCapture ??= () => {};
Element.prototype.releasePointerCapture ??= () => {};
Element.prototype.scrollIntoView ??= () => {};
