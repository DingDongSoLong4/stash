import { faBan, faMinus } from "@fortawesome/free-solid-svg-icons";
import React, { useCallback, useEffect, useState } from "react";
import { Button, ButtonGroup, Dropdown, DropdownButton } from "react-bootstrap";
import { useIntl } from "react-intl";
import { Icon, LoadingIndicator, SweatDrops } from "src/components/Shared";
import { useHotkeys } from "src/utils";

export interface IOCounterButtonProps {
  value: number;
  onIncrement: () => Promise<void>;
  onDecrement: () => Promise<void>;
  onReset: () => Promise<void>;
}

export const OCounterButton: React.FC<IOCounterButtonProps> = ({
  value,
  onIncrement,
  onDecrement,
  onReset,
}) => {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);

  const increment = useCallback(async () => {
    setLoading(true);
    await onIncrement();
    setLoading(false);
  }, [onIncrement]);

  async function decrement() {
    setLoading(true);
    await onDecrement();
    setLoading(false);
  }

  async function reset() {
    setLoading(true);
    await onReset();
    setLoading(false);
  }

  const hotkeys = useHotkeys();
  useEffect(() => {
    if (loading) return;

    return hotkeys.bind("o", () => {
      increment();
    });
  }, [hotkeys, loading, increment]);

  if (loading) return <LoadingIndicator message="" inline small />;

  const renderButton = () => (
    <Button
      className="minimal pr-1"
      onClick={increment}
      variant="secondary"
      title={intl.formatMessage({ id: "o_counter" })}
    >
      <SweatDrops />
      <span className="ml-2">{value}</span>
    </Button>
  );

  const maybeRenderDropdown = () => {
    if (value) {
      return (
        <DropdownButton
          as={ButtonGroup}
          title=" "
          variant="secondary"
          className="pl-0 show-carat"
        >
          <Dropdown.Item onClick={decrement}>
            <Icon icon={faMinus} />
            <span>Decrement</span>
          </Dropdown.Item>
          <Dropdown.Item onClick={reset}>
            <Icon icon={faBan} />
            <span>Reset</span>
          </Dropdown.Item>
        </DropdownButton>
      );
    }
  };

  return (
    <ButtonGroup className="o-counter">
      {renderButton()}
      {maybeRenderDropdown()}
    </ButtonGroup>
  );
};
