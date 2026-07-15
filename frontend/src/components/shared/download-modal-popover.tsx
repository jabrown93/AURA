import { PopoverHelp } from "@/components/shared/popover-help";

export interface DownloadModalPopoverProps {
  type:
    | "autodownload"
    | "add-to-db-only"
    | "add-to-queue-only"
    | "auto-add-new-collection-items"
    | "possible-future-types"
    | "force-preload-missing";
}

const downloadModalPopoverHelpText = {
  autodownload:
    "Auto Download will check periodically for new updates to this set. This is helpful if you want updates to this set to be downloaded automatically without having to manually check for updates. Auto Download runs based on the cron schedule you have set up in your settings. If you have not set up a cron schedule, Auto Download will not work.",
  "add-to-db-only":
    "Add to Database Only will not download anything right now. It will only add the set to your database. This is helpful for sets/images that you have already processed and just want to add the set to your database.",
  "add-to-queue-only":
    "Add to Queue will add the set to the download queue. This is helpful if you want to quickly add sets without waiting for downloads to finish. Downloads in the queue will be processed the same way as normal downloads. Download queue runs every 1 minute.",
  "auto-add-new-collection-items":
    "Auto Add New Collection Items will check periodically for new items added to this collection and automatically download the images for them",
  "possible-future-types":
    "These image types are not currently available in this set. Selecting them saves the type in your database for this set, so future auto-download checks can download them when they are added.",
  "force-preload-missing":
    "Force Preload Missing pre-stages season poster and title card images for seasons/episodes that are not yet on your server, as long as the show exists with at least one episode. Images are written to the Kometa asset directory only (they are not applied to the server), so Kometa picks them up automatically once the episodes are added. This applies to the download queue and auto-download flows, so it is only available when adding to the queue. Requires Kometa asset mode enabled (Plex only); it has no effect otherwise.",
};

const DownloadModalPopover: React.FC<DownloadModalPopoverProps> = ({ type }) => {
  const helpText = downloadModalPopoverHelpText[type as keyof typeof downloadModalPopoverHelpText];
  return (
    <div className="ml-auto">
      <PopoverHelp ariaLabel={`help-download-modal-${type}`}>
        <span>{helpText}</span>
      </PopoverHelp>
    </div>
  );
};

export default DownloadModalPopover;
