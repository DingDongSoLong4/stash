import {
  createMandatoryNumberCriterionOption,
  createMandatoryStringCriterionOption,
  createStringCriterionOption,
  createMandatoryTimestampCriterionOption,
  createDateCriterionOption,
} from "./criteria/criterion";
import { PerformerFavoriteCriterionOption } from "./criteria/favorite";
import { ImageIsMissingCriterionOption } from "./criteria/is-missing";
import { OrganizedCriterionOption } from "./criteria/organized";
import { PathCriterionOption } from "./criteria/path";
import { PerformersCriterionOption } from "./criteria/performers";
import { RatingCriterionOption } from "./criteria/rating";
import { ResolutionCriterionOption } from "./criteria/resolution";
import { OrientationCriterionOption } from "./criteria/orientation";
import { StudiosCriterionOption } from "./criteria/studios";
import {
  PerformerTagsCriterionOption,
  TagsCriterionOption,
} from "./criteria/tags";
import { ListFilterOptions, MediaSortByOptions } from "./filter-options";
import { DisplayMode } from "./types";
import { GalleriesCriterionOption } from "./criteria/galleries";

const defaultSortBy = "path";

const sortByOptions = [
  "o_counter",
  "filesize",
  "file_count",
  "date",
  ...MediaSortByOptions,
].map(ListFilterOptions.createSortBy);

const displayModeOptions = [DisplayMode.Grid, DisplayMode.Wall];
const criterionOptions = [
  createStringCriterionOption("title"),
  createStringCriterionOption("code", "scene_code"),
  createStringCriterionOption("details"),
  createStringCriterionOption("photographer"),
  createMandatoryStringCriterionOption("checksum", "media_info.checksum"),
  PathCriterionOption,
  GalleriesCriterionOption,
  OrganizedCriterionOption,
  createMandatoryNumberCriterionOption("o_counter"),
  ResolutionCriterionOption,
  OrientationCriterionOption,
  ImageIsMissingCriterionOption,
  TagsCriterionOption,
  RatingCriterionOption,
  createMandatoryNumberCriterionOption("tag_count"),
  PerformerTagsCriterionOption,
  PerformersCriterionOption,
  createMandatoryNumberCriterionOption("performer_count"),
  PerformerFavoriteCriterionOption,
  StudiosCriterionOption,
  createStringCriterionOption("url"),
  createDateCriterionOption("date"),
  createMandatoryNumberCriterionOption("file_count"),
  createMandatoryTimestampCriterionOption("created_at"),
  createMandatoryTimestampCriterionOption("updated_at"),
];
export const ImageListFilterOptions = new ListFilterOptions(
  defaultSortBy,
  sortByOptions,
  displayModeOptions,
  criterionOptions
);
