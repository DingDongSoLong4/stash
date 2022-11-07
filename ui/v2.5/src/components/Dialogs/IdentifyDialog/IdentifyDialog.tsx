import React, { useState, useEffect, useMemo } from "react";
import { Button, Form } from "react-bootstrap";
import {
  mutateMetadataIdentify,
  useConfiguration,
  useConfigureDefaults,
  useListSceneScrapers,
} from "src/core/StashService";
import { Icon, Modal, OperationButton } from "src/components/Shared";
import { useToast } from "src/hooks";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { withoutTypename } from "src/utils";
import {
  SCRAPER_PREFIX,
  STASH_BOX_PREFIX,
} from "src/components/Tagger/constants";
import { DirectorySelectionDialog } from "src/components/Settings/Tasks/DirectorySelectionDialog";
import { Manual } from "src/components/Help/Manual";
import { IScraperSource } from "./constants";
import { OptionsEditor } from "./Options";
import { SourcesEditor, SourcesList } from "./Sources";
import {
  faCogs,
  faFolderOpen,
  faQuestionCircle,
} from "@fortawesome/free-solid-svg-icons";

const autoTagScraperID = "builtin_autotag";

interface IIdentifyDialogProps {
  open: boolean;
  selectedIds?: string[];
  onClose: () => void;
}

export const IdentifyDialog: React.FC<IIdentifyDialogProps> = ({
  open,
  selectedIds,
  onClose,
}) => {
  function getDefaultOptions(): GQL.IdentifyMetadataOptionsInput {
    return {
      fieldOptions: [
        {
          field: "title",
          strategy: GQL.IdentifyFieldStrategy.Overwrite,
        },
      ],
      includeMalePerformers: true,
      setCoverImage: true,
      setOrganized: false,
    };
  }

  const [configureDefaults] = useConfigureDefaults();

  const [options, setOptions] = useState<GQL.IdentifyMetadataOptionsInput>(
    getDefaultOptions()
  );
  const [sources, setSources] = useState<IScraperSource[]>([]);
  const [editingSource, setEditingSource] = useState<IScraperSource>();
  const [paths, setPaths] = useState<string[]>([]);
  const [openDialog, setOpenDialog] = useState("");
  const [editingField, setEditingField] = useState(false);
  const [savingDefaults, setSavingDefaults] = useState(false);

  const intl = useIntl();
  const Toast = useToast();

  const { data: configData, error: configError } = useConfiguration();
  const { data: scraperData, error: scraperError } = useListSceneScrapers();

  const allSources = useMemo(() => {
    if (!configData || !scraperData) return;

    const ret: IScraperSource[] = [];

    ret.push(
      ...configData.configuration.general.stashBoxes.map((b, i) => {
        return {
          id: `${STASH_BOX_PREFIX}${i}`,
          displayName: `stash-box: ${b.name}`,
          stash_box_endpoint: b.endpoint,
        };
      })
    );

    const scrapers = scraperData.listSceneScrapers;

    const fragmentScrapers = scrapers.filter((s) =>
      s.scene?.supported_scrapes.includes(GQL.ScrapeType.Fragment)
    );

    ret.push(
      ...fragmentScrapers.map((s) => {
        return {
          id: `${SCRAPER_PREFIX}${s.id}`,
          displayName: s.name,
          scraper_id: s.id,
        };
      })
    );

    return ret;
  }, [configData, scraperData]);

  const availableSources = useMemo(() => {
    // only include scrapers not already present
    return (
      allSources?.filter((s) => !sources.some((ss) => ss.id === s.id)) ?? []
    );
  }, [allSources, sources]);

  const selectionStatus = useMemo(() => {
    if (selectedIds) {
      return (
        <Form.Group id="selected-identify-ids">
          <FormattedMessage
            id="config.tasks.identify.identifying_scenes"
            values={{
              num: selectedIds.length,
              scene: intl.formatMessage(
                {
                  id: "countables.scenes",
                },
                {
                  count: selectedIds.length,
                }
              ),
            }}
          />
          .
        </Form.Group>
      );
    }
    const message = paths.length ? (
      <div>
        <FormattedMessage id="config.tasks.identify.identifying_from_paths" />:
        <ul>
          {paths.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      </div>
    ) : (
      <span>
        <FormattedMessage
          id="config.tasks.identify.identifying_scenes"
          values={{
            num: intl.formatMessage({ id: "all" }),
            scene: intl.formatMessage(
              {
                id: "countables.scenes",
              },
              {
                count: 0,
              }
            ),
          }}
        />
        .
      </span>
    );

    return (
      <Form.Group className="dialog-selected-folders">
        <div>
          {message}
          <div>
            <Button
              title={intl.formatMessage({ id: "actions.select_folders" })}
              onClick={() => setOpenDialog("select_folders")}
            >
              <Icon icon={faFolderOpen} />
            </Button>
          </div>
        </div>
      </Form.Group>
    );
  }, [selectedIds, intl, paths]);

  useEffect(() => {
    if (!configData || !allSources) return;

    const { identify: identifyDefaults } = configData.configuration.defaults;

    if (identifyDefaults) {
      const mappedSources = identifyDefaults.sources
        .map((s) => {
          const found = allSources.find(
            (ss) =>
              ss.scraper_id === s.source.scraper_id ||
              ss.stash_box_endpoint === s.source.stash_box_endpoint
          );

          if (!found) return;

          const ret: IScraperSource = {
            ...found,
          };

          if (s.options) {
            const sourceOptions = withoutTypename(s.options);
            sourceOptions.fieldOptions = sourceOptions.fieldOptions?.map(
              withoutTypename
            );
            ret.options = sourceOptions;
          }

          return ret;
        })
        .filter((s) => s) as IScraperSource[];

      setSources(mappedSources);
      if (identifyDefaults.options) {
        const defaultOptions = withoutTypename(identifyDefaults.options);
        defaultOptions.fieldOptions = defaultOptions.fieldOptions?.map(
          withoutTypename
        );
        setOptions(defaultOptions);
      }
    } else {
      // default to first stash-box instance only
      const stashBox = allSources.find((s) => s.stash_box_endpoint);

      // add auto-tag as well
      const autoTag = allSources.find(
        (s) => s.id === `${SCRAPER_PREFIX}${autoTagScraperID}`
      );

      const newSources: IScraperSource[] = [];
      if (stashBox) {
        newSources.push(stashBox);
      }

      // sanity check - this should always be true
      if (autoTag) {
        // don't set organised by default
        const autoTagCopy = { ...autoTag };
        autoTagCopy.options = {
          setOrganized: false,
        };
        newSources.push(autoTagCopy);
      }

      setSources(newSources);
    }
  }, [allSources, configData]);

  if (configError || scraperError)
    return <div>{configError ?? scraperError}</div>;
  if (!allSources || !configData) return <div />;

  function makeIdentifyInput(): GQL.IdentifyMetadataInput {
    return {
      sources: sources.map((s) => {
        return {
          source: {
            scraper_id: s.scraper_id,
            stash_box_endpoint: s.stash_box_endpoint,
          },
          options: s.options,
        };
      }),
      options,
      sceneIDs: selectedIds,
      paths,
    };
  }

  function makeDefaultIdentifyInput() {
    const ret = makeIdentifyInput();
    const { sceneIDs, paths: _paths, ...withoutSpecifics } = ret;
    return withoutSpecifics;
  }

  async function onIdentify() {
    try {
      await mutateMetadataIdentify(makeIdentifyInput());

      Toast.success({
        content: intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.identify" }) }
        ),
      });
    } catch (e) {
      Toast.error(e);
    } finally {
      onClose();
    }
  }

  function onEditSource(s?: IScraperSource) {
    setEditingSource(s);
    setOpenDialog("edit_source");
  }

  function onSaveSource(s?: IScraperSource) {
    if (s) {
      let found = false;
      const newSources = sources.map((ss) => {
        if (ss.id === s.id) {
          found = true;
          return s;
        }
        return ss;
      });

      if (!found) {
        newSources.push(s);
      }

      setSources(newSources);
    }
    setOpenDialog("");
  }

  async function setAsDefault() {
    try {
      setSavingDefaults(true);
      await configureDefaults({
        variables: {
          input: {
            identify: makeDefaultIdentifyInput(),
          },
        },
      });

      Toast.success({
        content: intl.formatMessage(
          { id: "config.tasks.defaults_set" },
          { action: intl.formatMessage({ id: "actions.identify" }) }
        ),
      });
    } catch (e) {
      Toast.error(e);
    } finally {
      setSavingDefaults(false);
    }
  }

  return (
    <>
      <SourcesEditor
        editing={open && openDialog === "edit_source"}
        availableSources={availableSources}
        source={editingSource}
        saveSource={onSaveSource}
        defaultOptions={options}
      />
      <DirectorySelectionDialog
        open={open && openDialog === "select_folders"}
        allowEmpty
        initialPaths={paths}
        onClose={(p) => {
          if (p) {
            setPaths(p);
          }
          setOpenDialog("");
        }}
      />
      <Manual
        show={open && openDialog === "manual"}
        onClose={() => setOpenDialog("")}
        defaultActiveTab="Identify.md"
      />
      <Modal
        modalProps={{ size: "lg" }}
        show={open}
        icon={faCogs}
        header={intl.formatMessage({ id: "actions.identify" })}
        accept={{
          onClick: onIdentify,
          text: intl.formatMessage({ id: "actions.identify" }),
        }}
        cancel={{
          onClick: () => onClose(),
          text: intl.formatMessage({ id: "actions.cancel" }),
          variant: "secondary",
        }}
        disabled={editingField || savingDefaults || sources.length === 0}
        footerButtons={
          <OperationButton
            variant="secondary"
            disabled={editingField || savingDefaults}
            operation={setAsDefault}
          >
            <FormattedMessage id="actions.set_as_default" />
          </OperationButton>
        }
        leftFooterButtons={
          <Button
            title="Help"
            className="minimal help-button"
            onClick={() => setOpenDialog("manual")}
          >
            <Icon icon={faQuestionCircle} />
          </Button>
        }
      >
        <Form>
          {selectionStatus}
          <SourcesList
            sources={sources}
            setSources={(s) => setSources(s)}
            editSource={onEditSource}
            canAdd={sources.length < allSources.length}
          />
          <OptionsEditor
            options={options}
            setOptions={(o) => setOptions(o)}
            setEditingField={(v) => setEditingField(v)}
          />
        </Form>
      </Modal>
    </>
  );
};

export default IdentifyDialog;
