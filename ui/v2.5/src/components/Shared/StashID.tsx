import React, { useMemo } from "react";
import { StashId } from "src/core/generated-graphql";
import { ConfigurationContext } from "src/hooks/Config";
import { makeStashboxUrl } from "src/utils/stashbox";
import { ExternalLink } from "./ExternalLink";

export type LinkType = "performers" | "scenes" | "studios";

export const StashIDPill: React.FC<{
  stashID: StashId;
  linkType: LinkType;
}> = ({ stashID, linkType }) => {
  const { configuration } = React.useContext(ConfigurationContext);
  const stashBoxes = configuration?.general.stashBoxes;

  const { endpoint, stash_id } = stashID;

  const endpointName = useMemo(() => {
    const box = stashBoxes?.find((sb) => sb.endpoint === endpoint);
    return box?.name ?? endpoint;
  }, [stashBoxes, endpoint]);

  return (
    <span className="stash-id-pill" data-endpoint={endpointName}>
      {endpointName && <span>{endpointName}</span>}
      <ExternalLink href={makeStashboxUrl(endpoint, linkType, stash_id)}>
        {stash_id}
      </ExternalLink>
    </span>
  );
};
