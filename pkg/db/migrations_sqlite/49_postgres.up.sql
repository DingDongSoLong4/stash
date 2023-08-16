PRAGMA foreign_keys=OFF;

-- Make a few non-destructive changes so that the schema is more suited for postgres

-- Delete NULLs from images_tags

DELETE FROM `images_tags` WHERE `image_id` IS NULL OR `tag_id` IS NULL;

DROP INDEX `index_images_tags_on_tag_id`;

CREATE TABLE `images_tags_new` (
  `image_id` INTEGER NOT NULL,
  `tag_id` INTEGER NOT NULL,
  FOREIGN KEY(`image_id`) REFERENCES `images`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`image_id`, `tag_id`)
);
INSERT INTO `images_tags_new` SELECT * FROM `images_tags`;

DROP TABLE `images_tags`;
ALTER TABLE `images_tags_new` RENAME TO `images_tags`;

CREATE INDEX `index_images_tags_on_tag_id` ON `images_tags`(`tag_id`);


-- Delete NULLs from movies_scenes

DELETE FROM `movies_scenes` WHERE `movie_id` IS NULL OR `scene_id` IS NULL;

DROP INDEX `index_movies_scenes_on_movie_id`;

CREATE TABLE `movies_scenes_new` (
  `movie_id` INTEGER NOT NULL,
  `scene_id` INTEGER NOT NULL,
  `scene_index` TINYINT,
  FOREIGN KEY(`movie_id`) REFERENCES `movies`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`scene_id`) REFERENCES `scenes`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`movie_id`, `scene_id`)
);
INSERT INTO `movies_scenes_new` SELECT * FROM `movies_scenes`;

DROP TABLE `movies_scenes`;
ALTER TABLE `movies_scenes_new` RENAME TO `movies_scenes`;

CREATE INDEX `index_movies_scenes_on_scene_id` ON `movies_scenes`(`scene_id`);


-- Delete NULLs from performer_stash_ids

DELETE FROM `performer_stash_ids` WHERE `performer_id` IS NULL OR `endpoint` IS NULL OR `stash_id` IS NULL;

DROP INDEX `index_performer_stash_ids_on_performer_id`;

CREATE TABLE `performer_stash_ids_new` (
  `performer_id` INTEGER NOT NULL,
  `endpoint` VARCHAR(255) NOT NULL,
  `stash_id` VARCHAR(36) NOT NULL,
  FOREIGN KEY(`performer_id`) REFERENCES `performers`(`id`) ON DELETE CASCADE
);
INSERT INTO `performer_stash_ids_new` SELECT * FROM `performer_stash_ids`;

DROP TABLE `performer_stash_ids`;
ALTER TABLE `performer_stash_ids_new` RENAME TO `performer_stash_ids`;

CREATE INDEX `index_performer_stash_ids_on_performer_id` ON `performer_stash_ids`(`performer_id`);


-- Delete NULLs from performers_images

DELETE FROM `performers_images` WHERE `performer_id` IS NULL OR `image_id` IS NULL;

DROP INDEX `index_performers_images_on_performer_id`;

CREATE TABLE `performers_images_new` (
  `performer_id` INTEGER NOT NULL,
  `image_id` INTEGER NOT NULL,
  FOREIGN KEY(`performer_id`) REFERENCES `performers`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`image_id`) REFERENCES `images`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`image_id`, `performer_id`)
);
INSERT INTO `performers_images_new` SELECT * FROM `performers_images`;

DROP TABLE `performers_images`;
ALTER TABLE `performers_images_new` RENAME TO `performers_images`;

CREATE INDEX `index_performers_images_on_performer_id` ON `performers_images`(`performer_id`);


-- Delete NULLs from performers_scenes

DELETE FROM `performers_scenes` WHERE `performer_id` IS NULL OR `scene_id` IS NULL;

DROP INDEX `index_performers_scenes_on_performer_id`;

CREATE TABLE `performers_scenes_new` (
  `performer_id` INTEGER NOT NULL,
  `scene_id` INTEGER NOT NULL,
  FOREIGN KEY(`performer_id`) REFERENCES `performers`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`scene_id`) REFERENCES `scenes`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`scene_id`, `performer_id`)
);
INSERT INTO `performers_scenes_new` SELECT * FROM `performers_scenes`;

DROP TABLE `performers_scenes`;
ALTER TABLE `performers_scenes_new` RENAME TO `performers_scenes`;

CREATE INDEX `index_performers_scenes_on_performer_id` ON `performers_scenes`(`performer_id`);


-- Delete NULLs from scene_markers_tags

DELETE FROM `scene_markers_tags` WHERE `scene_marker_id` IS NULL OR `tag_id` IS NULL;

DROP INDEX `index_scene_markers_tags_on_tag_id`;

CREATE TABLE `scene_markers_tags_new` (
  `scene_marker_id` INTEGER NOT NULL,
  `tag_id` INTEGER NOT NULL,
  FOREIGN KEY(`scene_marker_id`) REFERENCES `scene_markers`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`scene_marker_id`, `tag_id`)
);
INSERT INTO `scene_markers_tags_new` SELECT * FROM `scene_markers_tags`;

DROP TABLE `scene_markers_tags`;
ALTER TABLE `scene_markers_tags_new` RENAME TO `scene_markers_tags`;

CREATE INDEX `index_scene_markers_tags_on_tag_id` ON `scene_markers_tags`(`tag_id`);


-- Delete NULLs from scenes_tags

DELETE FROM `scenes_tags` WHERE `scene_id` IS NULL OR `tag_id` IS NULL;

DROP INDEX `index_scenes_tags_on_tag_id`;

CREATE TABLE `scenes_tags_new` (
  `scene_id` INTEGER NOT NULL,
  `tag_id` INTEGER NOT NULL,
  FOREIGN KEY(`scene_id`) REFERENCES `scenes`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`scene_id`, `tag_id`)
);
INSERT INTO `scenes_tags_new` SELECT * FROM `scenes_tags`;

DROP TABLE `scenes_tags`;
ALTER TABLE `scenes_tags_new` RENAME TO `scenes_tags`;

CREATE INDEX `index_scenes_tags_on_tag_id` ON `scenes_tags`(`tag_id`);


-- Delete NULLs from studio_stash_ids

DELETE FROM `studio_stash_ids` WHERE `studio_id` IS NULL OR `endpoint` IS NULL OR `stash_id` IS NULL;

DROP INDEX `index_studio_stash_ids_on_studio_id`;

CREATE TABLE `studio_stash_ids_new` (
  `studio_id` INTEGER NOT NULL,
  `endpoint` VARCHAR(255) NOT NULL,
  `stash_id` VARCHAR(36) NOT NULL,
  FOREIGN KEY(`studio_id`) REFERENCES `studios`(`id`) ON DELETE CASCADE
);
INSERT INTO `studio_stash_ids_new` SELECT * FROM `studio_stash_ids`;

DROP TABLE `studio_stash_ids`;
ALTER TABLE `studio_stash_ids_new` RENAME TO `studio_stash_ids`;

CREATE INDEX `index_studio_stash_ids_on_studio_id` ON `studio_stash_ids`(`studio_id`);


-- Delete NULLs from tags_relations

DELETE FROM `tags_relations` WHERE `parent_id` IS NULL OR `child_id` IS NULL;

CREATE TABLE `tags_relations_new` (
  `parent_id` INTEGER NOT NULL,
  `child_id` INTEGER NOT NULL,
  FOREIGN KEY(`parent_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
  FOREIGN KEY(`child_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
  PRIMARY KEY(`parent_id`, `child_id`)
);
INSERT INTO `tags_relations_new` SELECT * FROM `tags_relations`;

DROP TABLE `tags_relations`;
ALTER TABLE `tags_relations_new` RENAME TO `tags_relations`;


-- rename indexes for consistency

DROP INDEX `index_fingerprint_type_fingerprint`;
CREATE INDEX `index_files_fingerprints_on_type_fingerprint` ON `files_fingerprints`(`type`, `fingerprint`);

DROP INDEX `index_galleries_files_file_id`;
CREATE INDEX `index_galleries_files_on_file_id` ON `galleries_files`(`file_id`);

DROP INDEX `index_scenes_files_file_id`;
CREATE INDEX `index_scenes_files_on_file_id` ON `scenes_files`(`file_id`);

DROP INDEX `performer_aliases_alias`;
CREATE INDEX `index_performer_aliases_on_alias` ON `performer_aliases`(`alias`);

DROP INDEX `performers_name_disambiguation_unique`;
CREATE UNIQUE INDEX `index_performers_on_name_disambiguation_unique` ON `performers`(`name`, `disambiguation`) WHERE `disambiguation` IS NOT NULL;

DROP INDEX `performers_name_unique`;
CREATE UNIQUE INDEX `index_performers_on_name_unique` ON `performers`(`name`) WHERE `disambiguation` IS NULL;

DROP INDEX `scene_urls_url`;
CREATE INDEX `index_scene_urls_on_url` ON `scene_urls`(`url`);

DROP INDEX `studio_aliases_alias_unique`;
CREATE UNIQUE INDEX `index_studio_aliases_on_alias_unique` ON `studio_aliases`(`alias`);

DROP INDEX `tag_aliases_alias_unique`;
CREATE UNIQUE INDEX `index_tag_aliases_on_alias_unique` ON `tag_aliases`(`alias`);

DROP INDEX `unique_index_galleries_files_on_primary`;
CREATE UNIQUE INDEX `index_galleries_files_on_gallery_id_primary_unique` ON `galleries_files`(`gallery_id`) WHERE `primary` = 1;

DROP INDEX `unique_index_images_files_on_primary`;
CREATE UNIQUE INDEX `index_images_files_on_image_id_primary_unique` ON `images_files`(`image_id`) WHERE `primary` = 1;

DROP INDEX `unique_index_scenes_files_on_primary`;
CREATE UNIQUE INDEX `index_scenes_files_on_scene_id_primary_unique` ON `scenes_files`(`scene_id`) WHERE `primary` = 1;


-- add new indexes

CREATE INDEX `index_studios_on_parent_id` ON `studios`(`parent_id`);

CREATE INDEX `index_tags_relations_on_child_id` ON `tags_relations`(`child_id`);


PRAGMA foreign_keys=ON;
