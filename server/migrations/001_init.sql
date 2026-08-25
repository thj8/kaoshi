-- 001_init.sql 答题系统初始表结构
-- 同时由 GORM AutoMigrate 兜底，此文件供 DBA 审阅 / 手动初始化

CREATE TABLE IF NOT EXISTS `users` (
  `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
  `username`      VARCHAR(64) DEFAULT NULL,
  `password_hash` VARCHAR(128) DEFAULT '',
  `nickname`      VARCHAR(64) NOT NULL,
  `created_at`    DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY `uk_users_username` (`username`),
  KEY `idx_users_nickname` (`nickname`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `quizzes` (
  `id`           BIGINT AUTO_INCREMENT PRIMARY KEY,
  `title`        VARCHAR(128) NOT NULL,
  `description`  VARCHAR(1024) DEFAULT '',
  `status`       VARCHAR(16) NOT NULL DEFAULT 'WAITING',
  `mode`         VARCHAR(16) NOT NULL DEFAULT 'normal',
  `total_time`   INT NOT NULL,
  `per_question_time` INT NOT NULL DEFAULT 30,
  `rush_enabled`     TINYINT(1) NOT NULL,
  `show_answer`      TINYINT(1) NOT NULL,
  `show_analysis`    TINYINT(1) NOT NULL,
  `show_ranking`     TINYINT(1) NOT NULL,
  `rush_winner_count` INT NOT NULL DEFAULT 1,
  `rush_time`         INT NOT NULL DEFAULT 10,
  `rush_answer_time`  INT NOT NULL DEFAULT 20,
  `rush_bonus_score`  INT NOT NULL DEFAULT 5,
  `rush_wrong_score`  INT NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `started_at` DATETIME(3) NULL,
  `ended_at`   DATETIME(3) NULL,
  KEY `idx_quizzes_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `questions` (
  `id`         BIGINT AUTO_INCREMENT PRIMARY KEY,
  `quiz_id`    BIGINT NOT NULL,
  `type`       VARCHAR(16) NOT NULL,
  `content`    VARCHAR(1024) NOT NULL,
  `answer`     VARCHAR(16) NOT NULL,
  `analysis`   VARCHAR(1024) DEFAULT '',
  `score`      INT NOT NULL DEFAULT 10,
  `required`   TINYINT(1) NOT NULL,
  `sort`       INT NOT NULL DEFAULT 0,
  `time_limit` INT NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  KEY `idx_questions_quiz_sort` (`quiz_id`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `question_options` (
  `id`          BIGINT AUTO_INCREMENT PRIMARY KEY,
  `question_id` BIGINT NOT NULL,
  `label`       VARCHAR(8) NOT NULL,
  `content`     VARCHAR(256) NOT NULL,
  `sort`        INT NOT NULL DEFAULT 0,
  KEY `idx_options_question` (`question_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `participants` (
  `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
  `quiz_id`       BIGINT NOT NULL,
  `user_id`       BIGINT NOT NULL,
  `score`         INT NOT NULL DEFAULT 0,
  `correct_count` INT NOT NULL DEFAULT 0,
  `wrong_count`   INT NOT NULL DEFAULT 0,
  `joined_at`     DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  `finished_at`   DATETIME(3) NULL,
  UNIQUE KEY `uk_participants_quiz_user` (`quiz_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `answers` (
  `id`           BIGINT AUTO_INCREMENT PRIMARY KEY,
  `quiz_id`      BIGINT NOT NULL,
  `question_id`  BIGINT NOT NULL,
  `user_id`      BIGINT NOT NULL,
  `answer`       VARCHAR(16) NOT NULL,
  `is_correct`   TINYINT(1) NOT NULL DEFAULT 0,
  `score`        INT NOT NULL DEFAULT 0,
  `duration`     INT NOT NULL DEFAULT 0,
  `submitted_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY `uk_answers` (`quiz_id`,`question_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `rush_records` (
  `id`          BIGINT AUTO_INCREMENT PRIMARY KEY,
  `quiz_id`     BIGINT NOT NULL,
  `question_id` BIGINT NOT NULL,
  `user_id`     BIGINT NOT NULL,
  `server_time` BIGINT NOT NULL,
  `rank`        INT NOT NULL DEFAULT 0,
  `score`       INT NOT NULL DEFAULT 0,
  `created_at`  DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY `uk_rush_records` (`quiz_id`,`question_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
