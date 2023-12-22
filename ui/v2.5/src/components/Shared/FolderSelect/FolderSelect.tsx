import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import {
  Button,
  InputGroup,
  Form,
  Collapse,
  OverlayTrigger,
  Tooltip,
} from "react-bootstrap";
import { Icon } from "../Icon";
import { LoadingIndicator } from "../LoadingIndicator";
import {
  faEllipsis,
  faTimes,
  faArrowUp,
} from "@fortawesome/free-solid-svg-icons";
import { useDebounce } from "src/hooks/debounce";
import TextUtils from "src/utils/text";
import cx from "classnames";
import { usePathUtils } from "src/hooks/paths";
import { useDirectory } from "src/core/StashService";

interface IProps {
  currentDirectory: string;
  onChangeDirectory: (value: string) => void;
  defaultDirectories?: string[];
  appendButton?: JSX.Element;
  collapsible?: boolean;
  quotePath?: boolean;
  hideError?: boolean;
}

export const FolderSelect: React.FC<IProps> = ({
  currentDirectory: _path,
  onChangeDirectory: _setPath,
  defaultDirectories,
  appendButton,
  collapsible = false,
  quotePath = false,
  hideError = false,
}) => {
  const path = quotePath ? TextUtils.stripQuotes(_path) : _path;

  function setPath(value: string) {
    if (quotePath && value.includes(" ")) {
      value = TextUtils.addQuotes(value);
    }
    _setPath(value);
  }

  const intl = useIntl();
  const { join: pathJoin } = usePathUtils();

  const [showBrowser, setShowBrowser] = useState(false);
  const [searchPath, setSearchPath] = useState(path);

  // skip the graphql query if we would be displaying defaultDirectories
  const skipSearch = !searchPath && defaultDirectories !== undefined;
  const { data, loading, error } = useDirectory(
    skipSearch ? undefined : searchPath
  );

  const { base: currentDir, parent, directories } = data?.directory ?? {};

  const [topLevel, setTopLevel] = useState(false);
  const [displayDirs, setDisplayDirs] = useState(directories);

  useEffect(() => {
    if (!path && defaultDirectories !== undefined) {
      setTopLevel(true);
      setDisplayDirs(defaultDirectories);
    } else if (directories !== undefined) {
      setTopLevel(false);
      setDisplayDirs(directories);
    }
  }, [defaultDirectories, directories, path]);

  const debouncedSetSearchPath = useDebounce(setSearchPath, 250);

  // debounce updates from path to searchPath
  useEffect(() => {
    debouncedSetSearchPath(path);
  }, [debouncedSetSearchPath, path]);

  function setDir(dir: string) {
    // ensure that dir ends in a slash, but only if dir is nonempty
    dir = pathJoin(dir, "");

    setPath(dir);
    debouncedSetSearchPath.cancel();
    setSearchPath(dir);
  }

  function goUp() {
    if (currentDir && defaultDirectories?.includes(currentDir)) {
      setDir("");
    } else if (parent) {
      setDir(parent);
    }
  }

  function renderFolderList() {
    return displayDirs?.map((dir) => {
      const dirPath =
        !topLevel && currentDir ? pathJoin(currentDir, dir) : dir;

      return (
        <li
          key={dir}
          className={cx("folder-list-item", { "top-level": topLevel })}
        >
          <Button
            variant="link"
            onClick={() => setDir(dirPath)}
            disabled={loading}
          >
            <span>{dir}</span>
          </Button>
        </li>
      );
    });
  }

  function renderContent() {
    if (!hideError && error !== undefined) {
      return <h5 className="mt-4 text-break">Error: {error.message}</h5>;
    }

    return (
      <Collapse in={!collapsible || showBrowser}>
        <ul className="folder-list">{renderFolderList()}</ul>
      </Collapse>
    );
  }

  const inputRef = useRef<HTMLInputElement>(null);

  // every time path changes, if the input is not focused,
  // scroll it all the way to the left
  useLayoutEffect(() => {
    const input = inputRef.current!;

    if (document.activeElement !== input) {
      input.scrollLeft = input.scrollWidth;
    }
  }, [path]);

  return (
    <>
      <InputGroup>
        <InputGroup.Prepend>
          <OverlayTrigger
            overlay={
              <Tooltip id="edit">
                <FormattedMessage id="setup.folder.up_dir" />
              </Tooltip>
            }
          >
            <Button
              variant="secondary"
              onClick={() => goUp()}
              disabled={loading || !parent}
            >
              <Icon icon={faArrowUp} />
            </Button>
          </OverlayTrigger>
        </InputGroup.Prepend>

        <Form.Control
          ref={inputRef}
          className="btn-secondary"
          placeholder={intl.formatMessage({ id: "setup.folder.file_path" })}
          onChange={(e) => setPath(e.currentTarget.value)}
          value={path}
          spellCheck={false}
        />

        {appendButton && <InputGroup.Append>{appendButton}</InputGroup.Append>}

        {collapsible && (
          <InputGroup.Append>
            <Button
              variant="secondary"
              onClick={() => setShowBrowser(!showBrowser)}
            >
              <Icon icon={faEllipsis} />
            </Button>
          </InputGroup.Append>
        )}

        {loading && (
          <InputGroup.Append>
            <LoadingIndicator small message="" />
          </InputGroup.Append>
        )}
        {error && !hideError && (
          <InputGroup.Append>
            <Icon icon={faTimes} color="red" className="ml-3" />
          </InputGroup.Append>
        )}
      </InputGroup>

      {renderContent()}
    </>
  );
};
