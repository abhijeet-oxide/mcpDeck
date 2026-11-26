import "@testing-library/jest-dom/vitest";

// jsdom lacks the pointer and scroll APIs Radix Select uses to open.
const proto = window.HTMLElement.prototype as unknown as Record<string, unknown>;
proto.hasPointerCapture ??= () => false;
proto.releasePointerCapture ??= () => {};
proto.setPointerCapture ??= () => {};
proto.scrollIntoView ??= () => {};
