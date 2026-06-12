/// <reference types="vite/client" />

interface Window {
  go?: {
    desktop?: {
      App?: Record<string, (...args: unknown[]) => Promise<unknown>>;
    };
    main?: {
      App?: Record<string, (...args: unknown[]) => Promise<unknown>>;
    };
  };
  runtime?: {
    EventsOn?: (name: string, callback: (event: unknown) => void) => () => void;
    EventsOff?: (name: string) => void;
  };
}
