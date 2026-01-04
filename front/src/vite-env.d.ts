/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_API_URL: string
    // more env variables...
  }
  
  interface ImportMeta {
    readonly env: ImportMetaEnv
  }

  declare namespace JSX {
    interface IntrinsicElements {
        'replay-web-page': React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement> & {
            source?: string;
            url?: string;
            embed?: string;
            'replayBase'?: string;
        };
    }
}
