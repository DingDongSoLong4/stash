BEGIN TRANSACTION;
SET CONSTRAINTS ALL DEFERRED;

CREATE TABLE "blobs" (
    "checksum" TEXT NOT NULL PRIMARY KEY,
    "blob" BYTEA
);

CREATE TABLE "files" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "basename" TEXT NOT NULL CHECK ("basename" != ''),
  "zip_file_id" INTEGER REFERENCES "files" ("id"),
  "parent_folder_id" INTEGER NOT NULL,
  "size" BIGINT NOT NULL,
  "mod_time" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "folders" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "path" TEXT NOT NULL,
  "parent_folder_id" INTEGER REFERENCES "folders" ("id") ON DELETE SET NULL,
  "mod_time" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "zip_file_id" INTEGER REFERENCES "files" ("id")
);

ALTER TABLE "files" ADD FOREIGN KEY ("parent_folder_id") REFERENCES "folders" ("id");

CREATE TABLE "files_fingerprints" (
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "type" TEXT NOT NULL,
  "fingerprint" TEXT NOT NULL,
  PRIMARY KEY ("file_id", "type", "fingerprint")
);

CREATE TABLE "video_files" (
  "file_id" INTEGER NOT NULL PRIMARY KEY REFERENCES "files" ("id") ON DELETE CASCADE,
  "duration" FLOAT NOT NULL,
  "video_codec" TEXT NOT NULL,
  "format" TEXT NOT NULL,
  "audio_codec" TEXT NOT NULL,
  "width" SMALLINT NOT NULL,
  "height" SMALLINT NOT NULL,
  "frame_rate" FLOAT NOT NULL,
  "bit_rate" INTEGER NOT NULL,
  "interactive" BOOLEAN NOT NULL DEFAULT FALSE,
  "interactive_speed" INTEGER
);

CREATE TABLE "video_captions" (
  "file_id" INTEGER NOT NULL REFERENCES "video_files" ("file_id") ON DELETE CASCADE,
  "language_code" TEXT NOT NULL,
  "caption_type" TEXT NOT NULL,
  "filename" TEXT NOT NULL,
  PRIMARY KEY ("file_id", "language_code", "caption_type")
);

CREATE TABLE "image_files" (
  "file_id" INTEGER NOT NULL PRIMARY KEY REFERENCES "files" ("id") ON DELETE CASCADE,
  "format" TEXT NOT NULL,
  "width" SMALLINT NOT NULL,
  "height" SMALLINT NOT NULL
);

CREATE TABLE "tags" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" TEXT,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "description" TEXT,
  "image_blob" TEXT REFERENCES "blobs" ("checksum")
);

CREATE TABLE "tags_relations" (
  "parent_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  "child_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("parent_id", "child_id")
);

CREATE TABLE "tag_aliases" (
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  "alias" TEXT NOT NULL,
  PRIMARY KEY ("tag_id", "alias")
);

CREATE TABLE "studios" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" TEXT,
  "url" TEXT,
  "parent_id" INTEGER DEFAULT NULL CHECK ("parent_id" <> "id") REFERENCES "studios" ("id") ON DELETE SET NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "details" TEXT,
  "rating" SMALLINT,
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "image_blob" TEXT REFERENCES "blobs" ("checksum")
);

CREATE TABLE "studio_stash_ids" (
  "studio_id" INTEGER NOT NULL REFERENCES "studios" ("id") ON DELETE CASCADE,
  "endpoint" TEXT NOT NULL,
  "stash_id" TEXT NOT NULL
);

CREATE TABLE "studio_aliases" (
  "studio_id" INTEGER NOT NULL REFERENCES "studios" ("id") ON DELETE CASCADE,
  "alias" TEXT NOT NULL,
  PRIMARY KEY ("studio_id", "alias")
);

CREATE TABLE "images" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" TEXT,
  "rating" SMALLINT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "o_counter" SMALLINT NOT NULL DEFAULT 0,
  "organized" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "url" TEXT,
  "date" DATE
);

CREATE TABLE "images_files" (
  "image_id" INTEGER NOT NULL REFERENCES "images" ("id") ON DELETE CASCADE,
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "primary" BOOLEAN NOT NULL,
  PRIMARY KEY ("image_id", "file_id")
);

CREATE TABLE "images_tags" (
  "image_id" INTEGER NOT NULL REFERENCES "images" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("image_id", "tag_id")
);

CREATE TABLE "galleries" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "folder_id" INTEGER REFERENCES "folders" ("id") ON DELETE SET NULL,
  "title" TEXT,
  "url" TEXT,
  "date" DATE,
  "details" TEXT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "rating" SMALLINT,
  "organized" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "galleries_files" (
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "primary" BOOLEAN NOT NULL,
  PRIMARY KEY ("gallery_id", "file_id")
);

CREATE TABLE "galleries_tags" (
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("gallery_id", "tag_id")
);

CREATE TABLE "galleries_images" (
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "image_id" INTEGER NOT NULL REFERENCES "images" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("gallery_id", "image_id")
);

CREATE TABLE "galleries_chapters" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" TEXT NOT NULL,
  "image_index" INTEGER NOT NULL,
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "scenes" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" TEXT,
  "details" TEXT,
  "date" DATE,
  "rating" SMALLINT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "o_counter" SMALLINT NOT NULL DEFAULT 0,
  "organized" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "code" TEXT,
  "director" TEXT,
  "resume_time" FLOAT NOT NULL DEFAULT 0,
  "last_played_at" TIMESTAMP DEFAULT NULL,
  "play_count" SMALLINT NOT NULL DEFAULT 0,
  "play_duration" FLOAT NOT NULL DEFAULT 0,
  "cover_blob" TEXT REFERENCES "blobs" ("checksum")
);

CREATE TABLE "scenes_files" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "primary" BOOLEAN NOT NULL,
  PRIMARY KEY ("scene_id", "file_id")
);

CREATE TABLE "scenes_tags" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "tag_id")
);

CREATE TABLE "scene_stash_ids" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "endpoint" TEXT NOT NULL,
  "stash_id" TEXT NOT NULL,
  PRIMARY KEY ("scene_id", "endpoint")
);

CREATE TABLE "scene_urls" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "position" INTEGER NOT NULL,
  "url" TEXT NOT NULL,
  PRIMARY KEY ("scene_id", "position", "url")
);

CREATE TABLE "scenes_galleries" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "gallery_id")
);

CREATE TABLE "scene_markers" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" TEXT NOT NULL,
  "seconds" FLOAT NOT NULL,
  "primary_tag_id" INTEGER NOT NULL REFERENCES "tags" ("id"),
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id"),
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "scene_markers_tags" (
  "scene_marker_id" INTEGER NOT NULL REFERENCES "scene_markers" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_marker_id", "tag_id")
);

CREATE TABLE "movies" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" TEXT NOT NULL,
  "aliases" TEXT,
  "duration" INTEGER,
  "date" DATE,
  "rating" SMALLINT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "director" TEXT,
  "synopsis" TEXT,
  "url" TEXT,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "front_image_blob" TEXT REFERENCES "blobs" ("checksum"),
  "back_image_blob" TEXT REFERENCES "blobs" ("checksum")
);

CREATE TABLE "movies_scenes" (
  "movie_id" INTEGER NOT NULL REFERENCES "movies" ("id") ON DELETE CASCADE,
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "scene_index" SMALLINT,
  PRIMARY KEY ("movie_id", "scene_id")
);

CREATE TABLE "performers" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" TEXT,
  "disambiguation" TEXT,
  "gender" TEXT,
  "url" TEXT,
  "twitter" TEXT,
  "instagram" TEXT,
  "birthdate" DATE,
  "ethnicity" TEXT,
  "country" TEXT,
  "eye_color" TEXT,
  "height" INTEGER,
  "measurements" TEXT,
  "fake_tits" TEXT,
  "career_length" TEXT,
  "tattoos" TEXT,
  "piercings" TEXT,
  "favorite" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "details" TEXT,
  "death_date" DATE,
  "hair_color" TEXT,
  "weight" INTEGER,
  "rating" SMALLINT,
  "penis_length" FLOAT,
  "circumcised" TEXT,
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "image_blob" TEXT REFERENCES "blobs" ("checksum")
);

CREATE TABLE "performer_stash_ids" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "endpoint" TEXT NOT NULL,
  "stash_id" TEXT NOT NULL
);

CREATE TABLE "performer_aliases" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "alias" TEXT NOT NULL,
  PRIMARY KEY ("performer_id", "alias")
);

CREATE TABLE "performers_tags" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("performer_id", "tag_id")
);

CREATE TABLE "performers_scenes" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "performer_id")
);

CREATE TABLE "performers_images" (
  "image_id" INTEGER NOT NULL REFERENCES "images" ("id") ON DELETE CASCADE,
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("image_id", "performer_id")
);

CREATE TABLE "performers_galleries" (
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("gallery_id", "performer_id")
);

CREATE TABLE "saved_filters" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "mode" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "filter" BYTEA NOT NULL
);

CREATE INDEX "index_files_on_basename" ON "files" ("basename");
CREATE UNIQUE INDEX "index_files_on_parent_folder_id_basename_unique" ON "files" ("parent_folder_id", "basename");
CREATE UNIQUE INDEX "index_files_zip_basename_unique" ON "files" ("zip_file_id", "parent_folder_id", "basename") WHERE "zip_file_id" IS NOT NULL;

CREATE UNIQUE INDEX "index_folders_on_path_unique" ON "folders" ("path");
CREATE INDEX "index_folders_on_parent_folder_id" ON "folders" ("parent_folder_id");
CREATE INDEX "index_folders_on_zip_file_id" ON "folders" ("zip_file_id") WHERE "zip_file_id" IS NOT NULL;

CREATE INDEX "index_files_fingerprints_on_type_fingerprint" ON "files_fingerprints" ("type", "fingerprint");

CREATE INDEX "index_tags_on_name" ON "tags" ("name");

CREATE INDEX "index_tags_relations_on_child_id" ON "tags_relations" ("child_id");

CREATE UNIQUE INDEX "index_tag_aliases_on_alias_unique" ON "tag_aliases" ("alias");

CREATE INDEX "index_studios_on_name" ON "studios" ("name");
CREATE INDEX "index_studios_on_parent_id" ON "studios" ("parent_id");

CREATE INDEX "index_studio_stash_ids_on_studio_id" ON "studio_stash_ids" ("studio_id");

CREATE UNIQUE INDEX "index_studio_aliases_on_alias_unique" ON "studio_aliases" ("alias");

CREATE INDEX "index_images_on_studio_id" ON "images" ("studio_id");

CREATE INDEX "index_images_files_on_file_id" ON "images_files" ("file_id");
CREATE UNIQUE INDEX "index_images_files_on_image_id_primary_unique" ON "images_files" ("image_id") WHERE "primary" = TRUE;

CREATE INDEX "index_images_tags_on_tag_id" ON "images_tags" ("tag_id");

CREATE UNIQUE INDEX "index_galleries_on_folder_id_unique" ON "galleries" ("folder_id");
CREATE INDEX "index_galleries_on_studio_id" ON "galleries" ("studio_id");

CREATE INDEX "index_galleries_files_on_file_id" ON "galleries_files" ("file_id");
CREATE UNIQUE INDEX "index_galleries_files_on_gallery_id_primary_unique" ON "galleries_files" ("gallery_id") WHERE "primary" = TRUE;

CREATE INDEX "index_galleries_tags_on_tag_id" ON "galleries_tags" ("tag_id");

CREATE INDEX "index_galleries_images_on_image_id" ON "galleries_images" ("image_id");

CREATE INDEX "index_galleries_chapters_on_gallery_id" on "galleries_chapters" ("gallery_id");

CREATE INDEX "index_scenes_on_studio_id" ON "scenes" ("studio_id");

CREATE INDEX "index_scenes_files_on_file_id" ON "scenes_files" ("file_id");
CREATE UNIQUE INDEX "index_scenes_files_on_scene_id_primary_unique" ON "scenes_files" ("scene_id") WHERE "primary" = TRUE;

CREATE INDEX "index_scenes_tags_on_tag_id" ON "scenes_tags" ("tag_id");

CREATE INDEX "index_scene_urls_on_url" on "scene_urls" ("url");

CREATE INDEX "index_scenes_galleries_on_gallery_id" ON "scenes_galleries" ("gallery_id");

CREATE INDEX "index_scene_markers_on_primary_tag_id" ON "scene_markers" ("primary_tag_id");
CREATE INDEX "index_scene_markers_on_scene_id" ON "scene_markers" ("scene_id");

CREATE INDEX "index_scene_markers_tags_on_tag_id" ON "scene_markers_tags" ("tag_id");

CREATE UNIQUE INDEX "index_movies_on_name_unique" ON "movies" ("name");
CREATE INDEX "index_movies_on_studio_id" ON "movies" ("studio_id");

CREATE INDEX "index_movies_scenes_on_scene_id" ON "movies_scenes" ("scene_id");

CREATE UNIQUE INDEX "index_performers_on_name_unique" ON "performers" ("name") WHERE "disambiguation" IS NULL;
CREATE UNIQUE INDEX "index_performers_on_name_disambiguation_unique" ON "performers" ("name", "disambiguation") WHERE "disambiguation" IS NOT NULL;

CREATE INDEX "index_performer_stash_ids_on_performer_id" ON "performer_stash_ids" ("performer_id");

CREATE INDEX "index_performer_aliases_on_alias" ON "performer_aliases" ("alias");

CREATE INDEX "index_performers_tags_on_tag_id" ON "performers_tags" ("tag_id");

CREATE INDEX "index_performers_scenes_on_performer_id" ON "performers_scenes" ("performer_id");

CREATE INDEX "index_performers_images_on_performer_id" ON "performers_images" ("performer_id");

CREATE INDEX "index_performers_galleries_on_performer_id" ON "performers_galleries" ("performer_id");

CREATE UNIQUE INDEX "index_saved_filters_on_mode_name_unique" ON "saved_filters" ("mode", "name");

CREATE COLLATION NOCASE (provider = icu, locale = '@colStrength=secondary', deterministic = false);
CREATE COLLATION NATURAL_CI (provider = icu, locale = '@colNumeric=yes;colStrength=secondary', deterministic = false);

COMMIT;
