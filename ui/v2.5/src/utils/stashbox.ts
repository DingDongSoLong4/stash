import * as GQL from "src/core/generated-graphql";

export function stashboxDisplayName(name: string, index: number) {
  return name || `Stash-Box #${index + 1}`;
}

export function getStashboxBase(endpoint?: string) {
  return endpoint?.match(/^(https?:\/\/.*?)\/graphql/)?.[1];
}

// builds a link to a stashbox page from an endpoint and URL path segments
export function makeStashboxUrl(
  endpoint?: string,
  ...segments: string[]
) {
  const base = getStashboxBase(endpoint);
  if (!base) return;

  return [base, ...segments].join("/")
}

export function stashIDKey(s: GQL.StashId) {
  return `${s.endpoint}:${s.stash_id}`;
}

// mergeStashIDs merges the src stash ID into the dest stash IDs.
// If the src stash ID is already in dest, the src stash ID overwrites the dest stash ID.
export function mergeStashIDs(dest: GQL.StashId[], src: GQL.StashId[]) {
  return dest
    .filter((i) => !src.some((j) => i.endpoint === j.endpoint))
    .concat(src);
}
