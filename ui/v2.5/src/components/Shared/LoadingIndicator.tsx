import React from "react";
import { Spinner } from "react-bootstrap";
import cx from "classnames";
import { useIntl } from "react-intl";

interface ILoadingProps {
  message?: string;
  inline?: boolean;
  small?: boolean;
  size?: "sm" | "md" | "lg";
  page?: boolean;
}

export const LoadingIndicator: React.FC<ILoadingProps> = ({
  message,
  inline = false,
  size = "lg",
  page = false,
}) => {
  const intl = useIntl();

  const text = intl.formatMessage({ id: "loading.generic" });

  return (
    <div
      className={cx("LoadingIndicator", `LoadingIndicator-${size}`, {
        "LoadingIndicator-page": page,
        inline,
      })}
    >
      <Spinner animation="border" role="status">
        <span className="sr-only">{text}</span>
      </Spinner>
      {message !== "" && (
        <span className="LoadingIndicator-message">{message ?? text}</span>
      )}
    </div>
  );
};
