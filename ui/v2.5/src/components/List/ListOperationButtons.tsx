import React, { useEffect } from "react";
import {
  Button,
  ButtonGroup,
  Dropdown,
  OverlayTrigger,
  Tooltip,
} from "react-bootstrap";
import { useHotkeys } from "src/utils";
import { FormattedMessage, useIntl } from "react-intl";
import { IconDefinition } from "@fortawesome/fontawesome-svg-core";
import { Icon } from "../Shared";
import {
  faEllipsisH,
  faPencilAlt,
  faTrash,
} from "@fortawesome/free-solid-svg-icons";

interface IListFilterOperation {
  text: string;
  onClick: () => void;
  isDisplayed?: () => boolean;
  icon?: IconDefinition;
  buttonVariant?: string;
}

interface IListOperationButtonsProps {
  onSelectAll?: () => void;
  onSelectNone?: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
  itemsSelected?: boolean;
  otherOperations?: IListFilterOperation[];
}

export const ListOperationButtons: React.FC<IListOperationButtonsProps> = ({
  onSelectAll,
  onSelectNone,
  onEdit,
  onDelete,
  itemsSelected,
  otherOperations,
}) => {
  const intl = useIntl();

  // set up hotkeys
  const hotkeys = useHotkeys();
  useEffect(() => {
    if (onSelectAll) {
      return hotkeys.bind("s a", () => onSelectAll());
    }
  }, [hotkeys, onSelectAll]);
  useEffect(() => {
    if (onSelectNone) {
      return hotkeys.bind("s n", () => onSelectNone());
    }
  }, [hotkeys, onSelectNone]);
  useEffect(() => {
    if (itemsSelected && onEdit) {
      return hotkeys.bind("e", () => onEdit());
    }
  }, [hotkeys, itemsSelected, onEdit]);
  useEffect(() => {
    if (itemsSelected && onDelete) {
      return hotkeys.bind("d d", () => onDelete());
    }
  }, [hotkeys, itemsSelected, onDelete]);

  function maybeRenderButtons() {
    const buttons = (otherOperations ?? []).filter((o) => {
      if (!o.icon) {
        return false;
      }

      if (!o.isDisplayed) {
        return true;
      }

      return o.isDisplayed();
    });
    if (itemsSelected) {
      if (onEdit) {
        buttons.push({
          icon: faPencilAlt,
          text: intl.formatMessage({ id: "actions.edit" }),
          onClick: onEdit,
        });
      }
      if (onDelete) {
        buttons.push({
          icon: faTrash,
          text: intl.formatMessage({ id: "actions.delete" }),
          onClick: onDelete,
          buttonVariant: "danger",
        });
      }
    }

    if (buttons.length > 0) {
      return (
        <ButtonGroup className="ml-2 mb-2">
          {buttons.map((button) => {
            return (
              <OverlayTrigger
                overlay={<Tooltip id="edit">{button.text}</Tooltip>}
                key={button.text}
              >
                <Button
                  variant={button.buttonVariant ?? "secondary"}
                  onClick={button.onClick}
                >
                  {button.icon ? <Icon icon={button.icon} /> : undefined}
                </Button>
              </OverlayTrigger>
            );
          })}
        </ButtonGroup>
      );
    }
  }

  function renderSelectAll() {
    if (onSelectAll) {
      return (
        <Dropdown.Item
          key="select-all"
          className="bg-secondary text-white"
          onClick={() => onSelectAll?.()}
        >
          <FormattedMessage id="actions.select_all" />
        </Dropdown.Item>
      );
    }
  }

  function renderSelectNone() {
    if (onSelectNone) {
      return (
        <Dropdown.Item
          key="select-none"
          className="bg-secondary text-white"
          onClick={() => onSelectNone?.()}
        >
          <FormattedMessage id="actions.select_none" />
        </Dropdown.Item>
      );
    }
  }

  function renderMore() {
    const options = [renderSelectAll(), renderSelectNone()].filter((o) => o);

    if (otherOperations) {
      otherOperations
        .filter((o) => {
          if (!o.isDisplayed) {
            return true;
          }

          return o.isDisplayed();
        })
        .forEach((o) => {
          options.push(
            <Dropdown.Item
              key={o.text}
              className="bg-secondary text-white"
              onClick={o.onClick}
            >
              {o.text}
            </Dropdown.Item>
          );
        });
    }

    if (options.length > 0) {
      return (
        <Dropdown>
          <Dropdown.Toggle variant="secondary" id="more-menu">
            <Icon icon={faEllipsisH} />
          </Dropdown.Toggle>
          <Dropdown.Menu className="bg-secondary text-white">
            {options}
          </Dropdown.Menu>
        </Dropdown>
      );
    }
  }

  return (
    <>
      {maybeRenderButtons()}

      <div className="mx-2 mb-2">{renderMore()}</div>
    </>
  );
};
