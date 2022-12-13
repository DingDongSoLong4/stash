import { Button, Modal } from "react-bootstrap";
import React, { useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { ImageInput } from "src/components/Shared/ImageInput";
import cx from "classnames";

interface IProps {
  objectName?: string;
  isNew: boolean;
  isEditing: boolean;
  onToggleEdit: () => void;
  onSave: () => void;
  saveDisabled?: boolean;
  onDelete?: () => void;
  onAutoTag?: () => void;
  onImageChange?: (event: React.FormEvent<HTMLInputElement>) => void;
  onImageChangeURL?: (url: string) => void;
  onImageClear?: () => void;
  onBackImageChange?: (event: React.FormEvent<HTMLInputElement>) => void;
  onBackImageChangeURL?: (url: string) => void;
  onBackImageClear?: () => void;
  acceptSVG?: boolean;
  customButtons?: JSX.Element;
  classNames?: string;
  children?: JSX.Element | JSX.Element[];
}

export const DetailsEditNavbar: React.FC<IProps> = (props: IProps) => {
  const intl = useIntl();
  const [isDeleteAlertOpen, setIsDeleteAlertOpen] = useState<boolean>(false);

  function renderEditButton() {
    if (props.isNew) return;
    return (
      <Button
        variant="primary"
        className="edit"
        onClick={() => props.onToggleEdit()}
      >
        {props.isEditing
          ? intl.formatMessage({ id: "actions.cancel" })
          : intl.formatMessage({ id: "actions.edit" })}
      </Button>
    );
  }

  function renderSaveButton() {
    if (!props.isEditing) return;

    return (
      <Button
        variant="success"
        className="save"
        disabled={props.saveDisabled}
        onClick={props.onSave}
      >
        <FormattedMessage id="actions.save" />
      </Button>
    );
  }

  function renderDeleteButton() {
    if (props.isNew || props.isEditing) return;
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

  function renderFrontImageInput() {
    if (!props.isEditing || !props.onImageChange) {
      return;
    }
    return (
      <>
        <ImageInput
          isEditing={props.isEditing}
          text={intl.formatMessage({ id: "actions.set_front_image" })}
          onImageChange={props.onImageChange}
          onImageURL={props.onImageChangeURL}
          acceptSVG={props.acceptSVG ?? false}
        />
        {props.onImageClear && (
          <div>
            <Button
              className="mr-2"
              variant="danger"
              onClick={props.onImageClear}
            >
              {props.onBackImageClear
                ? intl.formatMessage({ id: "actions.clear_front_image" })
                : intl.formatMessage({ id: "actions.clear_image" })}
            </Button>
          </div>
        )}
      </>
    );
  }

  function renderBackImageInput() {
    if (!props.isEditing || !props.onBackImageChange) {
      return;
    }
    return (
      <>
        <ImageInput
          isEditing={props.isEditing}
          text={intl.formatMessage({ id: "actions.set_back_image" })}
          onImageChange={props.onBackImageChange}
          onImageURL={props.onBackImageChangeURL}
        />
        {props.onBackImageClear && (
          <div>
            <Button
              className="mr-2"
              variant="danger"
              onClick={props.onBackImageClear}
            >
              {intl.formatMessage({ id: "actions.clear_back_image" })}
            </Button>
          </div>
        )}
      </>
    );
  }

  function renderAutoTagButton() {
    if (props.isNew || props.isEditing) return;

    if (props.onAutoTag) {
      return (
        <div>
          <Button
            variant="secondary"
            onClick={() => {
              if (props.onAutoTag) {
                props.onAutoTag();
              }
            }}
          >
            <FormattedMessage id="actions.auto_tag" />
          </Button>
        </div>
      );
    }
  }

  function renderDeleteAlert() {
    if (props.isNew || props.isEditing) return;

    return (
      <Modal show={isDeleteAlertOpen}>
        <Modal.Body>
          <FormattedMessage
            id="dialogs.delete_confirm"
            values={{ entityName: props.objectName }}
          />
        </Modal.Body>
        <Modal.Footer>
          <Button variant="danger" onClick={props.onDelete}>
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
    <div className={cx("details-edit", props.classNames)}>
      {renderEditButton()}
      {renderFrontImageInput()}
      {renderBackImageInput()}
      {renderAutoTagButton()}
      {props.customButtons}
      {renderSaveButton()}
      {renderDeleteButton()}
      {renderDeleteAlert()}
    </div>
  );
};
