import React, { useEffect, useState } from "react";
import { Button, Form, Col, Row } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import * as yup from "yup";
import { useImageUpdate } from "src/core/StashService";
import {
  PerformerSelect,
  TagSelect,
  StudioSelect,
  LoadingIndicator,
} from "src/components/Shared";
import { useToast } from "src/hooks";
import { FormUtils, useHotkeys } from "src/utils";
import { useFormik } from "formik";
import { Prompt } from "react-router-dom";
import { RatingStars } from "src/components/Scenes/SceneDetails/RatingStars";

interface IProps {
  image: GQL.ImageDataFragment;
  isVisible: boolean;
  onDelete: () => void;
}

export const ImageEditPanel: React.FC<IProps> = ({
  image,
  isVisible,
  onDelete,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  // Network state
  const [isLoading, setIsLoading] = useState(false);

  const [updateImage] = useImageUpdate();

  const schema = yup.object({
    title: yup.string().optional().nullable(),
    rating: yup.number().optional().nullable(),
    studio_id: yup.string().optional().nullable(),
    performer_ids: yup.array(yup.string().required()).optional().nullable(),
    tag_ids: yup.array(yup.string().required()).optional().nullable(),
  });

  const initialValues = {
    title: image.title ?? "",
    rating: image.rating ?? null,
    studio_id: image.studio?.id,
    performer_ids: (image.performers ?? []).map((p) => p.id),
    tag_ids: (image.tags ?? []).map((t) => t.id),
  };

  type InputValues = typeof initialValues;

  const formik = useFormik({
    initialValues,
    validationSchema: schema,
    onSubmit: (values) => onSave(getImageInput(values)),
  });

  // set up hotkeys
  const hotkeys = useHotkeys();
  useEffect(() => {
    if (isVisible && formik.dirty) {
      return hotkeys.bind("s s", () => formik.handleSubmit());
    }
  }, [hotkeys, isVisible, formik]);
  useEffect(() => {
    if (isVisible) {
      return hotkeys.bind("d d", () => onDelete());
    }
  }, [hotkeys, isVisible, onDelete]);
  useEffect(() => {
    if (!isVisible) return;

    function setRating(v: number | null) {
      formik.setFieldValue("rating", v);
    }

    hotkeys.bind("r 0", () => setRating(null));
    hotkeys.bind("r 1", () => setRating(1));
    hotkeys.bind("r 2", () => setRating(2));
    hotkeys.bind("r 3", () => setRating(3));
    hotkeys.bind("r 4", () => setRating(4));
    hotkeys.bind("r 5", () => setRating(5));

    return () => {
      hotkeys.unbind("r 0");
      hotkeys.unbind("r 1");
      hotkeys.unbind("r 2");
      hotkeys.unbind("r 3");
      hotkeys.unbind("r 4");
      hotkeys.unbind("r 5");
    };
  }, [hotkeys, isVisible, formik]);

  function getImageInput(input: InputValues): GQL.ImageUpdateInput {
    return {
      id: image.id,
      ...input,
    };
  }

  async function onSave(input: GQL.ImageUpdateInput) {
    setIsLoading(true);
    try {
      const result = await updateImage({
        variables: {
          input,
        },
      });
      if (result.data?.imageUpdate) {
        Toast.success({
          content: intl.formatMessage(
            { id: "toast.updated_entity" },
            { entity: intl.formatMessage({ id: "image" }).toLocaleLowerCase() }
          ),
        });
        formik.resetForm({ values: formik.values });
      }
    } catch (e) {
      Toast.error(e);
    }
    setIsLoading(false);
  }

  function renderTextField(field: string, title: string, placeholder?: string) {
    return (
      <Form.Group controlId={title} as={Row}>
        {FormUtils.renderLabel({
          title,
        })}
        <Col xs={9}>
          <Form.Control
            className="text-input"
            placeholder={placeholder ?? title}
            {...formik.getFieldProps(field)}
            isInvalid={!!formik.getFieldMeta(field).error}
          />
          <Form.Control.Feedback type="invalid">
            {formik.getFieldMeta(field).error}
          </Form.Control.Feedback>
        </Col>
      </Form.Group>
    );
  }

  if (isLoading) return <LoadingIndicator />;

  return (
    <div id="image-edit-details">
      <Prompt
        when={formik.dirty}
        message={intl.formatMessage({ id: "dialogs.unsaved_changes" })}
      />

      <Form noValidate onSubmit={formik.handleSubmit}>
        <div className="form-container row px-3 pt-3">
          <div className="col edit-buttons mb-3 pl-0">
            <Button
              className="edit-button"
              variant="primary"
              disabled={!formik.dirty}
              onClick={() => formik.submitForm()}
            >
              <FormattedMessage id="actions.save" />
            </Button>
            <Button
              className="edit-button"
              variant="danger"
              onClick={() => onDelete()}
            >
              <FormattedMessage id="actions.delete" />
            </Button>
          </div>
        </div>
        <div className="form-container row px-3">
          <div className="col-12 col-lg-6 col-xl-12">
            {renderTextField("title", intl.formatMessage({ id: "title" }))}
            <Form.Group controlId="rating" as={Row}>
              {FormUtils.renderLabel({
                title: intl.formatMessage({ id: "rating" }),
              })}
              <Col xs={9}>
                <RatingStars
                  value={formik.values.rating ?? undefined}
                  onSetRating={(value) =>
                    formik.setFieldValue("rating", value ?? null)
                  }
                />
              </Col>
            </Form.Group>

            <Form.Group controlId="studio" as={Row}>
              {FormUtils.renderLabel({
                title: intl.formatMessage({ id: "studio" }),
              })}
              <Col xs={9}>
                <StudioSelect
                  onSelect={(items) =>
                    formik.setFieldValue(
                      "studio_id",
                      items.length > 0 ? items[0]?.id : null
                    )
                  }
                  ids={formik.values.studio_id ? [formik.values.studio_id] : []}
                />
              </Col>
            </Form.Group>

            <Form.Group controlId="performers" as={Row}>
              {FormUtils.renderLabel({
                title: intl.formatMessage({ id: "performers" }),
                labelProps: {
                  column: true,
                  sm: 3,
                  xl: 12,
                },
              })}
              <Col sm={9} xl={12}>
                <PerformerSelect
                  isMulti
                  onSelect={(items) =>
                    formik.setFieldValue(
                      "performer_ids",
                      items.map((item) => item.id)
                    )
                  }
                  ids={formik.values.performer_ids}
                />
              </Col>
            </Form.Group>

            <Form.Group controlId="tags" as={Row}>
              {FormUtils.renderLabel({
                title: intl.formatMessage({ id: "tags" }),
                labelProps: {
                  column: true,
                  sm: 3,
                  xl: 12,
                },
              })}
              <Col sm={9} xl={12}>
                <TagSelect
                  isMulti
                  onSelect={(items) =>
                    formik.setFieldValue(
                      "tag_ids",
                      items.map((item) => item.id)
                    )
                  }
                  ids={formik.values.tag_ids}
                />
              </Col>
            </Form.Group>
          </div>
        </div>
      </Form>
    </div>
  );
};
