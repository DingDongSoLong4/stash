BEGIN TRANSACTION;
SET CONSTRAINTS ALL DEFERRED;

CREATE TABLE "blobs" (
    "checksum" VARCHAR(255) NOT NULL PRIMARY KEY,
    "blob" BYTEA
);

CREATE TABLE "files" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "basename" VARCHAR(255) NOT NULL CHECK ("basename" != ''),
  "zip_file_id" INTEGER REFERENCES "files" ("id"),
  "parent_folder_id" INTEGER NOT NULL,
  "size" INTEGER NOT NULL,
  "mod_time" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "folders" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "path" VARCHAR(255) NOT NULL,
  "parent_folder_id" INTEGER REFERENCES "folders" ("id") ON DELETE SET NULL,
  "mod_time" TIMESTAMP NOT NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "zip_file_id" INTEGER REFERENCES "files" ("id")
);

ALTER TABLE "files" ADD FOREIGN KEY ("parent_folder_id") REFERENCES "folders" ("id");

CREATE TABLE "files_fingerprints" (
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "type" VARCHAR(255) NOT NULL,
  "fingerprint" VARCHAR(128) NOT NULL,
  PRIMARY KEY ("file_id", "type", "fingerprint")
);

CREATE TABLE "video_files" (
  "file_id" INTEGER NOT NULL PRIMARY KEY REFERENCES "files" ("id") ON DELETE CASCADE,
  "duration" FLOAT NOT NULL,
  "video_codec" VARCHAR(255) NOT NULL,
  "format" VARCHAR(255) NOT NULL,
  "audio_codec" VARCHAR(255) NOT NULL,
  "width" SMALLINT NOT NULL,
  "height" SMALLINT NOT NULL,
  "frame_rate" FLOAT NOT NULL,
  "bit_rate" INTEGER NOT NULL,
  "interactive" BOOLEAN NOT NULL DEFAULT FALSE,
  "interactive_speed" INTEGER
);

CREATE TABLE "video_captions" (
  "file_id" INTEGER NOT NULL REFERENCES "video_files" ("file_id") ON DELETE CASCADE,
  "language_code" VARCHAR(255) NOT NULL,
  "filename" VARCHAR(255) NOT NULL,
  "caption_type" VARCHAR(255) NOT NULL,
  PRIMARY KEY ("file_id", "language_code", "caption_type")
);

CREATE TABLE "image_files" (
  "file_id" INTEGER NOT NULL PRIMARY KEY REFERENCES "files" ("id") ON DELETE CASCADE,
  "format" VARCHAR(255) NOT NULL,
  "width" SMALLINT NOT NULL,
  "height" SMALLINT NOT NULL
);

CREATE TABLE "tags" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" VARCHAR(255),
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "description" TEXT,
  "image_blob" VARCHAR(255) REFERENCES "blobs" ("checksum")
);

CREATE TABLE "tags_relations" (
  "parent_id" INTEGER REFERENCES "tags" ("id") ON DELETE CASCADE,
  "child_id" INTEGER REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("parent_id", "child_id")
);

CREATE TABLE "tag_aliases" (
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  "alias" VARCHAR(255) NOT NULL,
  PRIMARY KEY ("tag_id", "alias")
);

CREATE TABLE "studios" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" VARCHAR(255),
  "url" VARCHAR(255),
  "parent_id" INTEGER DEFAULT NULL CHECK ("parent_id" <> "id") REFERENCES "studios" ("id") ON DELETE SET NULL,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "details" TEXT,
  "rating" SMALLINT,
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "image_blob" VARCHAR(255) REFERENCES "blobs" ("checksum")
);

CREATE TABLE "studio_stash_ids" (
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE CASCADE,
  "endpoint" VARCHAR(255),
  "stash_id" VARCHAR(36)
);

CREATE TABLE "studio_aliases" (
  "studio_id" INTEGER NOT NULL REFERENCES "studios" ("id") ON DELETE CASCADE,
  "alias" VARCHAR(255) NOT NULL,
  PRIMARY KEY ("studio_id", "alias")
);

CREATE TABLE "images" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" VARCHAR(255),
  "rating" SMALLINT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "o_counter" SMALLINT NOT NULL DEFAULT 0,
  "organized" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "url" VARCHAR(255),
  "date" DATE
);

CREATE TABLE "images_files" (
  "image_id" INTEGER NOT NULL REFERENCES "images" ("id") ON DELETE CASCADE,
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "primary" BOOLEAN NOT NULL,
  PRIMARY KEY ("image_id", "file_id")
);

CREATE TABLE "images_tags" (
  "image_id" INTEGER REFERENCES "images" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("image_id", "tag_id")
);

CREATE TABLE "galleries" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "folder_id" INTEGER REFERENCES "folders" ("id") ON DELETE SET NULL,
  "title" VARCHAR(255),
  "url" VARCHAR(255),
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
  "title" varchar(255) not null,
  "image_index" INTEGER NOT NULL,
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "scenes" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" VARCHAR(255),
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
  "cover_blob" VARCHAR(255) REFERENCES "blobs" ("checksum")
);

CREATE TABLE "scenes_files" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "file_id" INTEGER NOT NULL REFERENCES "files" ("id") ON DELETE CASCADE,
  "primary" BOOLEAN NOT NULL,
  PRIMARY KEY ("scene_id", "file_id")
);

CREATE TABLE "scenes_tags" (
  "scene_id" INTEGER REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "tag_id")
);

CREATE TABLE "scene_stash_ids" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "endpoint" VARCHAR(255) NOT NULL,
  "stash_id" VARCHAR(36) NOT NULL,
  PRIMARY KEY ("scene_id", "endpoint")
);

CREATE TABLE "scene_urls" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "position" INTEGER NOT NULL,
  "url" VARCHAR(255) NOT NULL,
  PRIMARY KEY ("scene_id", "position", "url")
);

CREATE TABLE "scenes_galleries" (
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "gallery_id")
);

CREATE TABLE "scene_markers" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "title" VARCHAR(255) NOT NULL,
  "seconds" FLOAT NOT NULL,
  "primary_tag_id" INTEGER NOT NULL REFERENCES "tags" ("id"),
  "scene_id" INTEGER NOT NULL REFERENCES "scenes" ("id"),
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL
);

CREATE TABLE "scene_markers_tags" (
  "scene_marker_id" INTEGER REFERENCES "scene_markers" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_marker_id", "tag_id")
);

CREATE TABLE "movies" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" VARCHAR(255) NOT NULL,
  "aliases" VARCHAR(255),
  "duration" INTEGER,
  "date" DATE,
  "rating" SMALLINT,
  "studio_id" INTEGER REFERENCES "studios" ("id") ON DELETE SET NULL,
  "director" VARCHAR(255),
  "synopsis" TEXT,
  "url" VARCHAR(255),
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "front_image_blob" VARCHAR(255) REFERENCES "blobs" ("checksum"),
  "back_image_blob" VARCHAR(255) REFERENCES "blobs" ("checksum")
);

CREATE TABLE "movies_scenes" (
  "movie_id" INTEGER REFERENCES "movies" ("id") ON DELETE CASCADE,
  "scene_id" INTEGER REFERENCES "scenes" ("id") ON DELETE CASCADE,
  "scene_index" SMALLINT,
  PRIMARY KEY ("movie_id", "scene_id")
);

CREATE TABLE "performers" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" VARCHAR(255),
  "disambiguation" VARCHAR(255),
  "gender" VARCHAR(20),
  "url" VARCHAR(255),
  "twitter" VARCHAR(255),
  "instagram" VARCHAR(255),
  "birthdate" DATE,
  "ethnicity" VARCHAR(255),
  "country" VARCHAR(255),
  "eye_color" VARCHAR(255),
  "height" INTEGER,
  "measurements" VARCHAR(255),
  "fake_tits" VARCHAR(255),
  "career_length" VARCHAR(255),
  "tattoos" VARCHAR(255),
  "piercings" VARCHAR(255),
  "favorite" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_at" TIMESTAMP NOT NULL,
  "updated_at" TIMESTAMP NOT NULL,
  "details" TEXT,
  "death_date" DATE,
  "hair_color" VARCHAR(255),
  "weight" INTEGER,
  "rating" SMALLINT,
  "penis_length" FLOAT,
  "circumcised" VARCHAR(10),
  "ignore_auto_tag" BOOLEAN NOT NULL DEFAULT FALSE,
  "image_blob" VARCHAR(255) REFERENCES "blobs" ("checksum")
);

CREATE TABLE "performer_stash_ids" (
  "performer_id" INTEGER REFERENCES "performers" ("id") ON DELETE CASCADE,
  "endpoint" VARCHAR(255),
  "stash_id" VARCHAR(36)
);

CREATE TABLE "performer_aliases" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "alias" VARCHAR(255) NOT NULL,
  PRIMARY KEY ("performer_id", "alias")
);

CREATE TABLE "performers_tags" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "tag_id" INTEGER NOT NULL REFERENCES "tags" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("performer_id", "tag_id")
);

CREATE TABLE "performers_scenes" (
  "performer_id" INTEGER REFERENCES "performers" ("id") ON DELETE CASCADE,
  "scene_id" INTEGER REFERENCES "scenes" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("scene_id", "performer_id")
);

CREATE TABLE "performers_images" (
  "performer_id" INTEGER REFERENCES "performers" ("id") ON DELETE CASCADE,
  "image_id" INTEGER REFERENCES "images" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("image_id", "performer_id")
);

CREATE TABLE "performers_galleries" (
  "performer_id" INTEGER NOT NULL REFERENCES "performers" ("id") ON DELETE CASCADE,
  "gallery_id" INTEGER NOT NULL REFERENCES "galleries" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("gallery_id", "performer_id")
);

CREATE TABLE "saved_filters" (
  "id" SERIAL NOT NULL PRIMARY KEY,
  "name" VARCHAR(510) NOT NULL,
  "mode" VARCHAR(255) NOT NULL,
  "filter" BYTEA NOT NULL
);

CREATE INDEX "index_files_on_basename" ON "files" ("basename");
CREATE UNIQUE INDEX "index_files_zip_basename_unique" ON "files" ("zip_file_id", "parent_folder_id", "basename") WHERE "zip_file_id" IS NOT NULL;
CREATE UNIQUE INDEX "index_files_on_parent_folder_id_basename_unique" ON "files" ("parent_folder_id", "basename");

CREATE INDEX "index_folders_on_parent_folder_id" ON "folders" ("parent_folder_id");
CREATE INDEX "index_folders_on_zip_file_id" ON "folders" ("zip_file_id") WHERE "zip_file_id" IS NOT NULL;
CREATE UNIQUE INDEX "index_folders_on_path_unique" ON "folders" ("path");

CREATE INDEX "index_fingerprint_type_fingerprint" ON "files_fingerprints" ("type", "fingerprint");

CREATE INDEX "index_tags_on_name" ON "tags" ("name");

CREATE UNIQUE INDEX "tag_aliases_alias_unique" ON "tag_aliases" ("alias");

CREATE INDEX "index_studios_on_name" ON "studios" ("name");

CREATE INDEX "index_studio_stash_ids_on_studio_id" ON "studio_stash_ids" ("studio_id");

CREATE UNIQUE INDEX "studio_aliases_alias_unique" ON "studio_aliases" ("alias");

CREATE INDEX "index_images_on_studio_id" ON "images" ("studio_id");

CREATE INDEX "index_images_files_on_file_id" ON "images_files" ("file_id");
CREATE UNIQUE INDEX "unique_index_images_files_on_primary" ON "images_files" ("image_id") WHERE "primary" = TRUE;

CREATE INDEX "index_images_tags_on_tag_id" ON "images_tags" ("tag_id");

CREATE INDEX "index_galleries_on_studio_id" ON "galleries" ("studio_id");
CREATE UNIQUE INDEX "index_galleries_on_folder_id_unique" ON "galleries" ("folder_id");

CREATE INDEX "index_galleries_files_file_id" ON "galleries_files" ("file_id");
CREATE UNIQUE INDEX "unique_index_galleries_files_on_primary" ON "galleries_files" ("gallery_id") WHERE "primary" = TRUE;

CREATE INDEX "index_galleries_tags_on_tag_id" ON "galleries_tags" ("tag_id");

CREATE INDEX "index_galleries_images_on_image_id" ON "galleries_images" ("image_id");

CREATE INDEX "index_galleries_chapters_on_gallery_id" on "galleries_chapters" ("gallery_id");

CREATE INDEX "index_scenes_on_studio_id" ON "scenes" ("studio_id");

CREATE INDEX "index_scenes_files_file_id" ON "scenes_files" ("file_id");
CREATE UNIQUE INDEX "unique_index_scenes_files_on_primary" ON "scenes_files" ("scene_id") WHERE "primary" = TRUE;

CREATE INDEX "index_scenes_tags_on_tag_id" ON "scenes_tags" ("tag_id");

CREATE INDEX "scene_urls_url" on "scene_urls" ("url");

CREATE INDEX "index_scenes_galleries_on_gallery_id" ON "scenes_galleries" ("gallery_id");

CREATE INDEX "index_scene_markers_on_scene_id" ON "scene_markers" ("scene_id");
CREATE INDEX "index_scene_markers_on_primary_tag_id" ON "scene_markers" ("primary_tag_id");

CREATE INDEX "index_scene_markers_tags_on_tag_id" ON "scene_markers_tags" ("tag_id");

CREATE INDEX "index_movies_on_studio_id" ON "movies" ("studio_id");
CREATE UNIQUE INDEX "index_movies_on_name_unique" ON "movies" ("name");

CREATE INDEX "index_movies_scenes_on_movie_id" ON "movies_scenes" ("movie_id");

CREATE UNIQUE INDEX "performers_name_unique" ON "performers" ("name") WHERE "disambiguation" IS NULL;
CREATE UNIQUE INDEX "performers_name_disambiguation_unique" ON "performers" ("name", "disambiguation") WHERE "disambiguation" IS NOT NULL;

CREATE INDEX "index_performer_stash_ids_on_performer_id" ON "performer_stash_ids" ("performer_id");

CREATE INDEX "performer_aliases_alias" ON "performer_aliases" ("alias");

CREATE INDEX "index_performers_tags_on_tag_id" ON "performers_tags" ("tag_id");

CREATE INDEX "index_performers_scenes_on_performer_id" ON "performers_scenes" ("performer_id");

CREATE INDEX "index_performers_images_on_performer_id" ON "performers_images" ("performer_id");

CREATE INDEX "index_performers_galleries_on_performer_id" ON "performers_galleries" ("performer_id");

CREATE UNIQUE INDEX "index_saved_filters_on_mode_name_unique" ON "saved_filters" ("mode", "name");

CREATE COLLATION NOCASE (provider = icu, locale = '@colStrength=secondary', deterministic = false);
CREATE COLLATION NATURAL_CI (provider = icu, locale = '@colNumeric=yes;colStrength=secondary', deterministic = false);

COMMIT;
