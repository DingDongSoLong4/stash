import { Button } from "react-bootstrap";
import React from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { ImageInput } from "src/components/Shared/ImageInput";
import cx from "classnames";

interface IProps {
  isNew: boolean;
  isEditing: boolean;
  onToggleEdit?: () => void;
  onSave?: () => void;
  saveDisabled?: boolean;
  onDelete?: () => void;
  onAutoTag?: () => void;
  onFrontImageChange?: (event: React.FormEvent<HTMLInputElement>) => void;
  onFrontImageChangeURL?: (url: string) => void;
  onFrontImageClear?: () => void;
  onBackImageChange?: (event: React.FormEvent<HTMLInputElement>) => void;
  onBackImageChangeURL?: (url: string) => void;
  onBackImageClear?: () => void;
  acceptSVG?: boolean;
  customButtons?: JSX.Element;
  classNames?: string;
  children?: JSX.Element | JSX.Element[];
}

export const DetailsEditNavbar: React.FC<IProps> = ({
  isNew,
  isEditing,
  onToggleEdit,
  onSave,
  saveDisabled,
  onDelete,
  onAutoTag,
  onFrontImageChange,
  onFrontImageChangeURL,
  onFrontImageClear,
  onBackImageChange,
  onBackImageChangeURL,
  onBackImageClear,
  acceptSVG,
  customButtons,
  classNames,
}) => {
  const intl = useIntl();

  function renderEditButton() {
    if (isNew || !onToggleEdit) return;

    return (
      <Button variant="primary" className="edit" onClick={() => onToggleEdit()}>
        {isEditing
          ? intl.formatMessage({ id: "actions.cancel" })
          : intl.formatMessage({ id: "actions.edit" })}
      </Button>
    );
  }

  function renderSaveButton() {
    if (!isEditing || !onSave) return;

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
    if (isNew || isEditing || !onDelete) return;

    return (
      <Button variant="danger" className="delete" onClick={() => onDelete()}>
        <FormattedMessage id="actions.delete" />
      </Button>
    );
  }

  function renderFrontImageInput() {
    if (!isEditing || !onFrontImageChange) return;

    const text = onBackImageChange
      ? intl.formatMessage({ id: "actions.set_front_image" })
      : undefined;
    return (
      <ImageInput
        text={text}
        onImageChange={onFrontImageChange}
        onImageURL={onFrontImageChangeURL}
        acceptSVG={acceptSVG ?? false}
      />
    );
  }
  function renderFrontImageClear() {
    if (!isEditing || !onFrontImageClear) return;

    const text = onBackImageClear
      ? intl.formatMessage({ id: "actions.clear_front_image" })
      : intl.formatMessage({ id: "actions.clear_image" });
    return (
      <div>
        <Button
          className="mr-2"
          variant="danger"
          onClick={() => onFrontImageClear()}
        >
          {text}
        </Button>
      </div>
    );
  }

  function renderBackImageInput() {
    if (!isEditing || !onBackImageChange) return;

    return (
      <ImageInput
        text={intl.formatMessage({ id: "actions.set_back_image" })}
        onImageChange={onBackImageChange}
        onImageURL={onBackImageChangeURL}
      />
    );
  }

  function renderBackImageClear() {
    if (!isEditing || !onBackImageClear) return;

    return (
      <div>
        <Button
          className="mr-2"
          variant="danger"
          onClick={() => onBackImageClear()}
        >
          {intl.formatMessage({ id: "actions.clear_back_image" })}
        </Button>
      </div>
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

  return (
    <div className={cx("details-edit", classNames)}>
      {renderEditButton()}
      {renderFrontImageInput()}
      {renderFrontImageClear()}
      {renderBackImageInput()}
      {renderBackImageClear()}
      {renderAutoTagButton()}
      {customButtons}
      {renderSaveButton()}
      {renderDeleteButton()}
    </div>
  );
};
