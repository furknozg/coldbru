/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Origin prefix put before `/api/<service>`; empty (the default) keeps same-origin paths. */
  readonly VITE_API_BASE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
