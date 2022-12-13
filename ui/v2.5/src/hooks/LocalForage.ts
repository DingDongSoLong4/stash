import localForage from "localforage";
import isEqual from "lodash-es/isEqual";
import deepmerge from "deepmerge";
import { Dispatch, useEffect, useState } from "react";
import { ConfigImageLightboxInput } from "src/core/generated-graphql";

interface IInterfaceQueryConfig {
  disp?: number;
  itemsPerPage?: number;
  currentPage?: number;
}

type IQueryConfig = Record<string, IInterfaceQueryConfig>;

interface IInterfaceConfig {
  queryConfig: IQueryConfig;
  imageLightbox: ConfigImageLightboxInput;
}

export interface IChangelogConfig {
  versions: Record<string, boolean>;
}

interface ILocalForage<T> {
  data?: T;
  error: Error | null;
  loading: boolean;
}

type SetDataAction<T> = Partial<T> | ((prev: T) => Partial<T>);

const Cache: Record<string, {} | undefined> = {};

export function useLocalForage<T extends {}>(
  key: string,
  defaultValue: T = {} as T
): [ILocalForage<T>, Dispatch<SetDataAction<T>>] {
  const [error, setError] = useState<Error | null>(null);
  const [data, _setData] = useState(() => {
    const cachedData = Cache[key];
    if (cachedData) {
      return cachedData as T;
    } else {
      return defaultValue;
    }
  });
  const [loading, setLoading] = useState<boolean>();

  useEffect(() => {
    async function runAsync() {
      try {
        let parsed = await localForage.getItem<T>(key);
        if (typeof parsed === "string") {
          parsed = JSON.parse(parsed ?? "null");
        }
        if (parsed !== null) {
          _setData(parsed);
          Cache[key] = parsed;
        } else {
          _setData(defaultValue);
          Cache[key] = defaultValue;
        }
        setError(null);
      } catch (err) {
        if (err instanceof Error) setError(err);
        Cache[key] = defaultValue;
      } finally {
        setLoading(false);
      }
    }

    // Only run once
    if (loading === undefined) {
      setLoading(true);
      runAsync();
    }
  }, [loading, key, defaultValue]);

  const isLoading = loading || loading === undefined;

  function setData(value: SetDataAction<T>) {
    if (isLoading) return;

    let newValue;
    if (typeof value === "function") {
      newValue = value(data);
    } else {
      newValue = value;
    }

    // merge new value with previous value, overwriting arrays entirely
    const newData = deepmerge(data, newValue, {
      arrayMerge: (_, source) => source,
    });
    console.log({ data, newValue, newData });

    if (!isEqual(Cache[key], newData)) {
      _setData(newData);
      Cache[key] = newData;
      localForage.setItem(key, newData);
    }
  }

  return [{ data, error, loading: isLoading }, setData];
}

export const useInterfaceLocalForage = () =>
  useLocalForage<IInterfaceConfig>("interface");

export const useChangelogStorage = () =>
  useLocalForage<IChangelogConfig>("changelog");
