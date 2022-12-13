import React, { useMemo } from "react";
import queryString from "query-string";
import { FormattedMessage, useIntl } from "react-intl";
import { useLocation } from "react-router-dom";
import { GalleryEditPanel } from "./GalleryEditPanel";

const GalleryCreate: React.FC = () => {
  const intl = useIntl();
  const location = useLocation();
  const query = useMemo(() => queryString.parse(location.search), [location]);
  const gallery = {
    title: query.q ? String(query.q) : undefined,
  };

  return (
    <div className="row new-view">
      <div className="col-md-6">
        <h2>
          <FormattedMessage
            id="actions.create_entity"
            values={{ entityType: intl.formatMessage({ id: "gallery" }) }}
          />
        </h2>
        <GalleryEditPanel gallery={gallery} isVisible />
      </div>
    </div>
  );
};

export default GalleryCreate;
