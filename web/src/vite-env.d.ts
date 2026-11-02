/// <reference types="vite/client" />
declare module "*.css";
interface ImportMetaEnv {
  readonly HOARD_VERSION?: string;
  readonly HOARD_FIXTURE?: string;
  readonly HOARD_THEME?: string;
}
