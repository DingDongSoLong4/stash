import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Tabs, Tab, Col, Row } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { useParams, useHistory } from "react-router-dom";
import { Helmet } from "react-helmet";
import cx from "classnames";
import * as GQL from "src/core/generated-graphql";
import {
  useFindPerformer,
  usePerformerUpdate,
  usePerformerDestroy,
  mutateMetadataAutoTag,
} from "src/core/StashService";
import {
  Counter,
  CountryFlag,
  DetailsEditNavbar,
  ErrorMessage,
  Icon,
  LoadingIndicator,
  Modal,
} from "src/components/Shared";
import { useLightbox, useToast } from "src/hooks";
import { ConfigurationContext } from "src/hooks/Config";
import { TextUtils, useHotkeys } from "src/utils";
import { RatingStars } from "src/components/Scenes/SceneDetails/RatingStars";
import { PerformerDetailsPanel } from "./PerformerDetailsPanel";
import { PerformerScenesPanel } from "./PerformerScenesPanel";
import { PerformerGalleriesPanel } from "./PerformerGalleriesPanel";
import { PerformerMoviesPanel } from "./PerformerMoviesPanel";
import { PerformerImagesPanel } from "./PerformerImagesPanel";
import { PerformerEditPanel } from "./PerformerEditPanel";
import { PerformerSubmitButton } from "./PerformerSubmitButton";
import GenderIcon from "../GenderIcon";
import {
  faCamera,
  faDove,
  faHeart,
  faLink,
  faTrashAlt,
} from "@fortawesome/free-solid-svg-icons";
import { IUIConfig } from "src/core/config";

interface IProps {
  performer: GQL.PerformerDataFragment;
}
interface IPerformerParams {
  tab?: string;
}

const PerformerPage: React.FC<IProps> = ({ performer }) => {
  const Toast = useToast();
  const history = useHistory();
  const intl = useIntl();
  const { tab = "details" } = useParams<IPerformerParams>();

  // Configuration settings
  const { configuration } = React.useContext(ConfigurationContext);
  const abbreviateCounter =
    (configuration?.ui as IUIConfig)?.abbreviateCounters ?? false;

  // Editing state
  const [isEditing, setIsEditing] = useState<boolean>(false);
  const [isDeleteAlertOpen, setIsDeleteAlertOpen] = useState<boolean>(false);

  const [imagePreview, setImagePreview] = useState<string | null>();
  const [imageEncoding, setImageEncoding] = useState<boolean>(false);

  // if undefined then get the existing image
  // if null then get the default (no) image
  // otherwise get the set image
  const activeImage =
    imagePreview === undefined
      ? performer.image_path ?? ""
      : imagePreview ?? `${performer.image_path}&default=true`;
  const lightboxImages = useMemo(
    () => [{ paths: { thumbnail: activeImage, image: activeImage } }],
    [activeImage]
  );

  const showLightbox = useLightbox({
    images: lightboxImages,
  });

  const [updatePerformer] = usePerformerUpdate();
  const [deletePerformer, { loading: isDestroying }] = usePerformerDestroy();

  const activeTabKey =
    tab === "scenes" ||
    tab === "galleries" ||
    tab === "images" ||
    tab === "movies"
      ? tab
      : "details";
  const setActiveTabKey = useCallback(
    (newTab: string | null) => {
      if (tab !== newTab) {
        const tabParam = newTab === "details" ? "" : `/${newTab}`;
        history.replace(`/performers/${performer.id}${tabParam}`);
      }
    },
    [history, performer.id, tab]
  );

  const onImageChange = (image?: string | null) => setImagePreview(image);

  const onImageEncoding = (isEncoding = false) => setImageEncoding(isEncoding);

  async function onAutoTag() {
    try {
      await mutateMetadataAutoTag({ performers: [performer.id] });
      Toast.success({
        content: intl.formatMessage({ id: "toast.started_auto_tagging" }),
      });
    } catch (e) {
      Toast.error(e);
    }
  }

  const setFavorite = useCallback(
    (v: boolean) => {
      if (performer.id) {
        updatePerformer({
          variables: {
            input: {
              id: performer.id,
              favorite: v,
            },
          },
        });
      }
    },
    [performer.id, updatePerformer]
  );

  const setRating = useCallback(
    (v: number | null) => {
      if (performer.id) {
        updatePerformer({
          variables: {
            input: {
              id: performer.id,
              rating: v,
            },
          },
        });
      }
    },
    [performer.id, updatePerformer]
  );

  // set up hotkeys
  const hotkeys = useHotkeys();
  useEffect(() => {
    return hotkeys.bind("f", () => setFavorite(!performer.favorite));
  }, [hotkeys, performer.favorite, setFavorite]);
  useEffect(() => {
    if (isEditing) return;

    hotkeys.bind("a", () => setActiveTabKey("details"));
    hotkeys.bind("s", () => setActiveTabKey("scenes"));
    hotkeys.bind("l", () => setActiveTabKey("galleries"));
    hotkeys.bind("i", () => setActiveTabKey("images"));
    hotkeys.bind("m", () => setActiveTabKey("movies"));

    return () => {
      hotkeys.unbind("a");
      hotkeys.unbind("s");
      hotkeys.unbind("l");
      hotkeys.unbind("i");
      hotkeys.unbind("m");
    };
  }, [hotkeys, isEditing, setActiveTabKey]);
  useEffect(() => {
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
  }, [hotkeys, setRating]);

  async function onDelete() {
    try {
      await deletePerformer({ variables: { id: performer.id } });
    } catch (e) {
      Toast.error(e);
    }

    // redirect to performers page
    history.push("/performers");
  }

  function renderDeleteAlert() {
    return (
      <Modal
        show={isDeleteAlertOpen}
        icon={faTrashAlt}
        accept={{
          text: intl.formatMessage({ id: "actions.delete" }),
          variant: "danger",
          onClick: onDelete,
        }}
        cancel={{ onClick: () => setIsDeleteAlertOpen(false) }}
      >
        <p>
          <FormattedMessage
            id="dialogs.delete_confirm"
            values={{
              entityName:
                performer.name ??
                intl.formatMessage({ id: "performer" }).toLocaleLowerCase(),
            }}
          />
        </p>
      </Modal>
    );
  }

  const renderTabs = () => (
    <React.Fragment>
      <Col>
        <Row xs={8}>
          <DetailsEditNavbar
            onToggleEdit={() => setIsEditing(!isEditing)}
            onDelete={() => setIsDeleteAlertOpen(true)}
            onAutoTag={onAutoTag}
            isNew={false}
            isEditing={false}
            classNames="mb-2"
            customButtons={
              <div>
                <PerformerSubmitButton performer={performer} />
              </div>
            }
          ></DetailsEditNavbar>
        </Row>
      </Col>
      <Tabs
        activeKey={activeTabKey}
        onSelect={setActiveTabKey}
        id="performer-details"
        unmountOnExit
      >
        <Tab eventKey="details" title={intl.formatMessage({ id: "details" })}>
          <PerformerDetailsPanel performer={performer} />
        </Tab>
        <Tab
          eventKey="scenes"
          title={
            <React.Fragment>
              {intl.formatMessage({ id: "scenes" })}
              <Counter
                abbreviateCounter={abbreviateCounter}
                count={performer.scene_count ?? 0}
              />
            </React.Fragment>
          }
        >
          <PerformerScenesPanel performer={performer} />
        </Tab>
        <Tab
          eventKey="galleries"
          title={
            <React.Fragment>
              {intl.formatMessage({ id: "galleries" })}
              <Counter
                abbreviateCounter={abbreviateCounter}
                count={performer.gallery_count ?? 0}
              />
            </React.Fragment>
          }
        >
          <PerformerGalleriesPanel performer={performer} />
        </Tab>
        <Tab
          eventKey="images"
          title={
            <React.Fragment>
              {intl.formatMessage({ id: "images" })}
              <Counter
                abbreviateCounter={abbreviateCounter}
                count={performer.image_count ?? 0}
              />
            </React.Fragment>
          }
        >
          <PerformerImagesPanel performer={performer} />
        </Tab>
        <Tab
          eventKey="movies"
          title={
            <React.Fragment>
              {intl.formatMessage({ id: "movies" })}
              <Counter
                abbreviateCounter={abbreviateCounter}
                count={performer.movie_count ?? 0}
              />
            </React.Fragment>
          }
        >
          <PerformerMoviesPanel performer={performer} />
        </Tab>
      </Tabs>
    </React.Fragment>
  );

  function renderTabsOrEditPanel() {
    if (isEditing) {
      return (
        <PerformerEditPanel
          performer={performer}
          isVisible={isEditing}
          isNew={false}
          onImageChange={onImageChange}
          onImageEncoding={onImageEncoding}
          onCancelEditing={() => setIsEditing(false)}
        />
      );
    } else {
      return renderTabs();
    }
  }

  function maybeRenderAge() {
    if (performer?.birthdate) {
      // calculate the age from birthdate. In future, this should probably be
      // provided by the server
      return (
        <div>
          <span className="age">
            {TextUtils.age(performer.birthdate, performer.death_date)}
          </span>
          <span className="age-tail">
            {" "}
            <FormattedMessage id="years_old" />
          </span>
        </div>
      );
    }
  }

  function maybeRenderAliases() {
    if (performer?.aliases) {
      return (
        <div>
          <span className="alias-head">
            <FormattedMessage id="also_known_as" />{" "}
          </span>
          <span className="alias">{performer.aliases}</span>
        </div>
      );
    }
  }

  const renderClickableIcons = () => (
    <span className="name-icons">
      <Button
        className={cx(
          "minimal",
          performer.favorite ? "favorite" : "not-favorite"
        )}
        onClick={() => setFavorite(!performer.favorite)}
      >
        <Icon icon={faHeart} />
      </Button>
      {performer.url && (
        <Button className="minimal icon-link">
          <a
            href={TextUtils.sanitiseURL(performer.url)}
            className="link"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Icon icon={faLink} />
          </a>
        </Button>
      )}
      {performer.twitter && (
        <Button className="minimal icon-link">
          <a
            href={TextUtils.sanitiseURL(
              performer.twitter,
              TextUtils.twitterURL
            )}
            className="twitter"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Icon icon={faDove} />
          </a>
        </Button>
      )}
      {performer.instagram && (
        <Button className="minimal icon-link">
          <a
            href={TextUtils.sanitiseURL(
              performer.instagram,
              TextUtils.instagramURL
            )}
            className="instagram"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Icon icon={faCamera} />
          </a>
        </Button>
      )}
    </span>
  );

  if (isDestroying)
    return (
      <LoadingIndicator
        message={`Deleting performer ${performer.id}: ${performer.name}`}
      />
    );

  return (
    <div id="performer-page" className="row">
      <Helmet>
        <title>{performer.name}</title>
      </Helmet>

      <div className="performer-image-container col-md-4 text-center">
        {imageEncoding ? (
          <LoadingIndicator message="Encoding image..." />
        ) : (
          <Button variant="link" onClick={() => showLightbox()}>
            <img
              className="performer"
              src={activeImage}
              alt={intl.formatMessage({ id: "performer" })}
            />
          </Button>
        )}
      </div>
      <div className="col-md-8">
        <div className="row">
          <div className="performer-head col">
            <h2>
              <GenderIcon
                gender={performer.gender}
                className="gender-icon mr-2 flag-icon"
              />
              <CountryFlag country={performer.country} className="mr-2" />
              {performer.name}
              {renderClickableIcons()}
            </h2>
            <RatingStars
              value={performer.rating ?? undefined}
              onSetRating={(value) => setRating(value ?? null)}
            />
            {maybeRenderAliases()}
            {maybeRenderAge()}
          </div>
        </div>
        <div className="performer-body">
          <div className="performer-tabs">{renderTabsOrEditPanel()}</div>
        </div>
      </div>
      {renderDeleteAlert()}
    </div>
  );
};

const PerformerLoader: React.FC = () => {
  const { id } = useParams<{ id?: string }>();
  const { data, loading, error } = useFindPerformer(id ?? "");

  if (loading) return <LoadingIndicator />;
  if (error) return <ErrorMessage error={error.message} />;
  if (!data?.findPerformer)
    return <ErrorMessage error={`No performer found with id ${id}.`} />;

  return <PerformerPage performer={data.findPerformer} />;
};

export default PerformerLoader;
