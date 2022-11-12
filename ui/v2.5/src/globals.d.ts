// eslint-disable-next-line no-var
declare var STASH_BASE_URL: string;
declare module "*.md";
declare module "intersection-observer";
declare module "mousetrap-pause";

/* eslint-disable  @typescript-eslint/naming-convention */
interface ImportMetaEnv extends Readonly<Record<string, string>> {
  readonly VITE_APP_GITHASH?: string;
  readonly VITE_APP_STASH_VERSION?: string;
  readonly VITE_APP_DATE?: string;
  readonly VITE_APP_PLATFORM_PORT?: string;
  readonly VITE_APP_HTTPS?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
/* eslint-enable  @typescript-eslint/no-unused-vars */
