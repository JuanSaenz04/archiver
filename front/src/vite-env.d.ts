/// <reference types="vite/client" />

declare namespace JSX {
  interface IntrinsicElements {
    "replay-web-page": React.DetailedHTMLProps<
      React.HTMLAttributes<HTMLElement>,
      HTMLElement
    > & {
      source?: string;
      url?: string;
      embed?: string;
      replayBase?: string;
    };
  }
}
