import React, { useEffect, useState } from "react";
import { Form } from "react-bootstrap";
import { Modal } from "src/components/Shared";
import { faCogs } from "@fortawesome/free-solid-svg-icons";
import { useIntl } from "react-intl";
import { MarkdownPage } from "../Shared/MarkdownPage";
import { Module } from "src/docs/en/ReleaseNotes";

interface IReleaseNotesDialog {
  notes: Module[];
  onClose: () => void;
}

export const ReleaseNotesDialog: React.FC<IReleaseNotesDialog> = ({
  notes,
  onClose,
}) => {
  const intl = useIntl();
  const [closed, setClosed] = useState(false);
  const [displayNotes, setDisplayNotes] = useState(notes);

  useEffect(() => {
    // Don't update displayNotes if closed - prevents dialog
    // from going blank while still fading out
    if (!closed) {
      setDisplayNotes(notes);
    }
  }, [closed, notes]);

  function onHide() {
    // don't open again after initial dialog is closed
    setClosed(true);
    onClose();
  }

  return (
    <Modal
      show={!closed && displayNotes.length !== 0}
      icon={faCogs}
      onHide={onHide}
      header={intl.formatMessage({ id: "release_notes" })}
      accept={{
        onClick: onHide,
        text: intl.formatMessage({ id: "actions.close" }),
      }}
    >
      <Form>
        {displayNotes.map((n, i) => (
          <MarkdownPage page={n} key={i} />
        ))}
      </Form>
    </Modal>
  );
};

export default ReleaseNotesDialog;
