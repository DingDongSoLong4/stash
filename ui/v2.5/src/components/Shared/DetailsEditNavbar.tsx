import { Button, Modal } from "react-bootstrap";
import React, { useEffect, useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { ImageInput } from "src/components/Shared/ImageInput";
import { useHotkeys } from "src/utils";
import cx from "classnames";

interface IProps {
  objectName?: string;
  isNew: boolean;
  isEditing: boolean;
  onToggleEdit: () => void;
  onSave: () => void;
  saveDisabled?: boolean;
  onDelete: () => void;
  onAutoTag?: () => void;
  onImageChange: (event: React.FormEvent<HTMLInputElement>) => void;
  onBackImageChange?: (event: React.FormEvent<HTMLInputElement>) => void;
  onImageChangeURL?: (url: string) => void;
  onBackImageChangeURL?: (url: string) => void;
  onClearImage?: () => void;
  onClearBackImage?: () => void;
  acceptSVG?: boolean;
  customButtons?: JSX.Element;
  classNames?: string;
  children?: JSX.Element | JSX.Element[];
}

export const DetailsEditNavbar: React.FC<IProps> = ({
  objectName,
  isNew,
  isEditing,
  onToggleEdit,
  onSave,
  saveDisabled,
  onDelete,
  onAutoTag,
  onImageChange,
  onBackImageChange,
  onImageChangeURL,
  onBackImageChangeURL,
  onClearImage,
  onClearBackImage,
  acceptSVG,
  customButtons,
  classNames,
}) => {
  const intl = useIntl();
  const [isDeleteAlertOpen, setIsDeleteAlertOpen] = useState<boolean>(false);

  // set up hotkeys
  const hotkeys = useHotkeys();
  useEffect(() => {
    if (!onToggleEdit) return;
    return hotkeys.bind("e", () => onToggleEdit());
  }, [hotkeys, onToggleEdit]);
  useEffect(() => {
    if (!isEditing || saveDisabled) return;
    return hotkeys.bind("s s", () => onSave());
  }, [hotkeys, isEditing, saveDisabled, onSave]);
  useEffect(() => {
    if (isNew || isEditing || !onDelete) return;
    return hotkeys.bind("d d", () => onDelete());
  }, [hotkeys, isEditing, isNew, onDelete]);

  function renderEditButton() {
    if (isNew) return;
    return (
      <Button variant="primary" className="edit" onClick={() => onToggleEdit()}>
        {isEditing
          ? intl.formatMessage({ id: "actions.cancel" })
          : intl.formatMessage({ id: "actions.edit" })}
      </Button>
    );
  }

  function renderSaveButton() {
    if (!isEditing) return;

    return (
      <Button
        variant="success"
        className="save"
        disabled={saveDisabled}
        onClick={() => onSave()}
      >
        <FormattedMessage id="actions.save" />
      </Button>
    );
  }

  function renderDeleteButton() {
    if (isNew || isEditing) return;
    return (
      <Button
        variant="danger"
        className="delete"
        onClick={() => setIsDeleteAlertOpen(true)}
      >
        <FormattedMessage id="actions.delete" />
      </Button>
    );
  }

  function renderBackImageInput() {
    if (!isEditing || !onBackImageChange) {
      return;
    }
    return (
      <ImageInput
        isEditing={isEditing}
        text={intl.formatMessage({ id: "actions.set_back_image" })}
        onImageChange={onBackImageChange}
        onImageURL={onBackImageChangeURL}
      />
    );
  }

  function renderAutoTagButton() {
    if (isNew || isEditing) return;

    if (onAutoTag) {
      return (
        <div>
          <Button variant="secondary" onClick={() => onAutoTag()}>
            <FormattedMessage id="actions.auto_tag" />
          </Button>
        </div>
      );
    }
  }

  function renderDeleteAlert() {
    return (
      <Modal show={isDeleteAlertOpen}>
        <Modal.Body>
          <FormattedMessage
            id="dialogs.delete_confirm"
            values={{ entityName: objectName }}
          />
        </Modal.Body>
        <Modal.Footer>
          <Button variant="danger" onClick={onDelete}>
            <FormattedMessage id="actions.delete" />
          </Button>
          <Button
            variant="secondary"
            onClick={() => setIsDeleteAlertOpen(false)}
          >
            <FormattedMessage id="actions.cancel" />
          </Button>
        </Modal.Footer>
      </Modal>
    );
  }

  return (
    <div className={cx("details-edit", classNames)}>
      {renderEditButton()}
      <ImageInput
        isEditing={isEditing}
        text={
          onBackImageChange
            ? intl.formatMessage({ id: "actions.set_front_image" })
            : undefined
        }
        onImageChange={onImageChange}
        onImageURL={onImageChangeURL}
        acceptSVG={acceptSVG ?? false}
      />
      {isEditing && onClearImage ? (
        <div>
          <Button
            className="mr-2"
            variant="danger"
            onClick={() => onClearImage()}
          >
            {onClearBackImage
              ? intl.formatMessage({ id: "actions.clear_front_image" })
              : intl.formatMessage({ id: "actions.clear_image" })}
          </Button>
        </div>
      ) : null}
      {renderBackImageInput()}
      {isEditing && onClearBackImage ? (
        <div>
          <Button
            className="mr-2"
            variant="danger"
            onClick={() => onClearBackImage()}
          >
            {intl.formatMessage({ id: "actions.clear_back_image" })}
          </Button>
        </div>
      ) : null}
      {renderAutoTagButton()}
      {customButtons}
      {renderSaveButton()}
      {renderDeleteButton()}
      {renderDeleteAlert()}
    </div>
  );
};
