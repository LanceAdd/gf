-- Seata MySQL Driver Test Schema
-- This file contains additional test schema used by unit tests

-- 1. 创建测试用户表
CREATE TABLE IF NOT EXISTS `test_user` (
    `id` INT(11) NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    `name` VARCHAR(50) NOT NULL COMMENT '用户名',
    `age` INT(11) NOT NULL DEFAULT 0 COMMENT '年龄',
    `balance` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '余额',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试用户表';

-- 2. 创建测试订单表
CREATE TABLE IF NOT EXISTS `test_order` (
    `id` INT(11) NOT NULL AUTO_INCREMENT COMMENT '订单ID',
    `user_id` INT(11) NOT NULL COMMENT '用户ID',
    `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '订单金额',
    `status` VARCHAR(20) NOT NULL DEFAULT 'PENDING' COMMENT '订单状态',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试订单表';

-- 3. 插入测试数据
INSERT INTO `test_user` (`id`, `name`, `age`, `balance`) VALUES
(1, 'Alice', 25, 1000.00),
(2, 'Bob', 30, 500.00),
(3, 'Charlie', 35, 2000.00);

-- 4. 清理旧的 undo log（测试前）
TRUNCATE TABLE `undo_log`;