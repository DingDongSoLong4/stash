import React, { useRef, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { SettingSection } from "./SettingSection";
import * as GQL from "src/core/generated-graphql";
import { SettingModal } from "./Inputs";

export interface IStashBoxModal {
  show: boolean;
  value: GQL.StashBoxInput;
  close: (v?: GQL.StashBoxInput) => void;
}

export const StashBoxModal: React.FC<IStashBoxModal> = ({ show, value, close }) => {
  const intl = useIntl();
  const endpoint = useRef<HTMLInputElement>(null);
  const apiKey = useRef<HTMLInputElement>(null);

  const [validate, { data, loading }] = GQL.useValidateStashBoxLazyQuery({
    fetchPolicy: "network-only",
  });

  const handleValidate = () => {
    validate({
      variables: {
        input: {
          endpoint: endpoint.current?.value ?? "",
          api_key: apiKey.current?.value ?? "",
          name: "test",
        },
      },
    });
  };

  return (
    <SettingModal<GQL.StashBoxInput>
      show={show}
      headingID="config.stashbox.title"
      value={value}
      renderField={(v, setValue) => (
        <>
          <Form.Group id="stashbox-name">
            <h6>
              {intl.formatMessage({
                id: "config.stashbox.name",
              })}
            </h6>
            <Form.Control
              placeholder={intl.formatMessage({ id: "config.stashbox.name" })}
              className="text-input stash-box-name"
              value={v?.name}
              isValid={(v?.name?.length ?? 0) > 0}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setValue({ ...v!, name: e.currentTarget.value })
              }
            />
          </Form.Group>

          <Form.Group id="stashbox-name">
            <h6>
              {intl.formatMessage({
                id: "config.stashbox.graphql_endpoint",
              })}
            </h6>
            <Form.Control
              placeholder={intl.formatMessage({
                id: "config.stashbox.graphql_endpoint",
              })}
              className="text-input stash-box-endpoint"
              value={v?.endpoint}
              isValid={(v?.endpoint?.length ?? 0) > 0}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setValue({ ...v!, endpoint: e.currentTarget.value.trim() })
              }
              ref={endpoint}
            />
          </Form.Group>

          <Form.Group id="stashbox-name">
            <h6>
              {intl.formatMessage({
                id: "config.stashbox.api_key",
              })}
            </h6>
            <Form.Control
              placeholder={intl.formatMessage({
                id: "config.stashbox.api_key",
              })}
              className="text-input stash-box-apikey"
              value={v?.api_key}
              isValid={(v?.api_key?.length ?? 0) > 0}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setValue({ ...v!, api_key: e.currentTarget.value.trim() })
              }
              ref={apiKey}
            />
          </Form.Group>

          <Form.Group>
            <Button
              disabled={loading}
              onClick={handleValidate}
              className="mr-3"
            >
              Test Credentials
            </Button>
            {data && (
              <b
                className={
                  data.validateStashBoxCredentials?.valid
                    ? "text-success"
                    : "text-danger"
                }
              >
                {data.validateStashBoxCredentials?.status}
              </b>
            )}
          </Form.Group>
        </>
      )}
      close={close}
    />
  );
};

interface IStashBoxSetting {
  value: GQL.StashBoxInput[];
  onChange: (v: GQL.StashBoxInput[]) => void;
}

export const StashBoxSetting: React.FC<IStashBoxSetting> = ({
  value,
  onChange,
}) => {
  const [showModal, setShowModal] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number>(-1);

  function onEdit(index: number) {
    setEditingIndex(index);
    setShowModal(true);
  }

  function onDelete(index: number) {
    onChange(value.filter((v, i) => i !== index));
  }

  function onNew() {
    setEditingIndex(-1);
    setShowModal(true);
  }

  function getEditingValue() {
    return value[editingIndex] ?? {
      endpoint: "",
      api_key: "",
      name: "",
    };
  }

  function onClose(v: GQL.StashBoxInput) {
    if (editingIndex !== -1) {
      onChange(
        value.map((vv, index) => {
          if (index === editingIndex) {
            return v;
          }
          return vv;
        })
      );
    } else {
      onChange([...value, v]);
    }
    setEditingIndex(-1);
  }

  return (
    <SettingSection
      id="stash-boxes"
      headingID="config.stashbox.title"
      subHeadingID="config.stashbox.description"
    >
      <StashBoxModal
        show={showModal}
        value={getEditingValue()}
        close={(v) => {
          if (v) onClose(v);
          setShowModal(false);
        }}
      />

      {value.map((b, index) => (
        // eslint-disable-next-line react/no-array-index-key
        <div key={index} className="setting">
          <div>
            <h3>{b.name ?? `#${index}`}</h3>
            <div className="value">{b.endpoint ?? ""}</div>
          </div>
          <div>
            <Button onClick={() => onEdit(index)}>
              <FormattedMessage id="actions.edit" />
            </Button>
            <Button variant="danger" onClick={() => onDelete(index)}>
              <FormattedMessage id="actions.delete" />
            </Button>
          </div>
        </div>
      ))}
      <div className="setting">
        <div />
        <div>
          <Button onClick={() => onNew()}>
            <FormattedMessage id="actions.add" />
          </Button>
        </div>
      </div>
    </SettingSection>
  );
};
