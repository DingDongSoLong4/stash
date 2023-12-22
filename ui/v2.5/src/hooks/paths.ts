import { useMemo } from "react";
import { useSystemStatus } from "src/core/StashService";

export function usePathUtils() {
  const { data: systemStatus } = useSystemStatus();
  const status = systemStatus?.systemStatus;

  return useMemo(() => {
    // default to Unix behaviour while status is loading
    const windows = status?.os === "windows";

    const SEPARATOR = windows ? "\\" : "/";

    const PWD = windows ? "%CD%" : "$PWD";
    const WORKING_DIR = status?.workingDir || PWD;

    const HOME = windows ? "%USERPROFILE%" : "$HOME";
    const HOME_DIR = status?.homeDir || HOME;

    // fairly rudimentary path joining
    function join(...paths: string[]) {
      let path = "";
      for (const p of paths) {
        if (!path || path.endsWith(SEPARATOR)) {
          path += p;
        } else {
          path += SEPARATOR + p;
        }
      }
      return path;
    }

    // simply returns everything preceding the last path separator
    function dir(path: string) {
      const lastSep = path.lastIndexOf(SEPARATOR);
      if (lastSep === -1) return "";
      return path.slice(0, lastSep);
    }

    return {
      SEPARATOR,
      PWD,
      WORKING_DIR,
      HOME,
      HOME_DIR,
      join,
      dir,
    };
  }, [status]);
}
