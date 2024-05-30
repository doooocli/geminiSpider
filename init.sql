CREATE TABLE `gen` (
    `id` bigint NOT NULL AUTO_INCREMENT,
    `key` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
    `proxy` varchar(200) DEFAULT NULL,
    `content` longtext,
    `prompt` text,
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB;

CREATE TABLE `keys` (
    `id` bigint NOT NULL AUTO_INCREMENT,
    `key` varchar(64) NOT NULL DEFAULT '',
    `status` tinyint NOT NULL DEFAULT '0' COMMENT '0-禁用1-启用',
    `error` text,
    `comment` varchar(200) NOT NULL DEFAULT '',
    `active_ts` bigint NOT NULL DEFAULT '0' COMMENT '启用时间戳/秒',
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `un_key` (`key`)
) ENGINE=InnoDB;