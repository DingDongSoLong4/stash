import React, { useRef, useState } from "react";
import { OverlayTrigger, Tooltip } from "react-bootstrap";
import { Placement } from "react-bootstrap/Overlay";
import cx from "classnames";

const CLASSNAME = "TruncatedText";
const CLASSNAME_TOOLTIP = `${CLASSNAME}-tooltip`;

interface ITruncatedTextProps {
  text?: JSX.Element | string | null;
  lineCount?: number;
  placement?: Placement;
  delay?: number;
  className?: string;
}

export const TruncatedText: React.FC<ITruncatedTextProps> = ({
  text,
  className,
  lineCount = 1,
  placement = "bottom",
  delay = 1000,
}) => {
  const [showTooltip, setShowTooltip] = useState(false);
  const target = useRef<HTMLDivElement>(null);

  if (!text) return null;

  function onToggle(nextShow: boolean) {
    const element = target.current;

    // Check if visible size is smaller than the content size
    if (
      element && (
        element.offsetWidth < element.scrollWidth ||
        element.offsetHeight + 10 < element.scrollHeight
      )
    ) {
      setShowTooltip(nextShow);
    } else {
      setShowTooltip(false);
    }
  }

  return (
    <OverlayTrigger
      show={showTooltip}
      placement={placement}
      flip
      delay={{ show: delay, hide: 0 }}
      onToggle={onToggle}
      overlay={
        <Tooltip id={CLASSNAME} className={CLASSNAME_TOOLTIP}>
          {text}
        </Tooltip>
      }
    >
      <div
        className={cx(CLASSNAME, className)}
        style={{ WebkitLineClamp: lineCount }}
        ref={target}
      >
        {text}
      </div>
    </OverlayTrigger>
  );
};
