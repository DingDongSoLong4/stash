import React from "react";
import { Popover, OverlayProps, OverlayTrigger } from "react-bootstrap";

interface IHoverPopover {
  enterDelay?: number;
  leaveDelay?: number;
  content: JSX.Element[] | JSX.Element | string;
  className?: string;
  placement?: OverlayProps["placement"];
  onOpen?: () => void;
  onClose?: () => void;
}

export const HoverPopover: React.FC<IHoverPopover> = ({
  enterDelay = 0,
  leaveDelay = 400,
  content,
  children,
  className,
  placement = "top",
  onOpen,
  onClose,
}) => {
  function onToggle(nextShow: boolean) {
    if (nextShow) {
      onOpen?.();
    } else {
      onClose?.();
    }
  }

  return (
    <OverlayTrigger
      placement={placement}
      delay={{show: enterDelay, hide: leaveDelay}}
      flip
      overlay={(
        <Popover
          id="popover"
          className="hover-popover-content"
        >
          {content}
        </Popover>
      )}
      onToggle={onToggle}
    >
      <div
        className={className}
      >
        {children}
      </div>
    </OverlayTrigger>
  );
};
