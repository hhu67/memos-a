CREATE TABLE `folder` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `parent_id` INT DEFAULT NULL,
  `name` VARCHAR(256) NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX `idx_folder_creator_id` ON `folder`(`creator_id`, `id`);
CREATE INDEX `idx_folder_parent_id` ON `folder`(`parent_id`);

ALTER TABLE `memo` ADD COLUMN `title` VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE `memo` ADD COLUMN `folder_id` INT DEFAULT NULL;

CREATE INDEX `idx_memo_folder_id` ON `memo`(`folder_id`, `row_status`, `created_ts`, `id`);
