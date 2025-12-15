-- =====================================================
-- Seata Demo Database Initialization Script
-- =====================================================

-- 1. 创建数据库
DROP DATABASE IF EXISTS seata_demo;
CREATE DATABASE seata_demo DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

USE seata_demo;

-- 2. 创建账户表
DROP TABLE IF EXISTS accounts;
CREATE TABLE accounts (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '账户ID',
    user_id INT NOT NULL COMMENT '用户ID',
    balance DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '账户余额',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账户表';

-- 3. 创建订单表
DROP TABLE IF EXISTS orders;
CREATE TABLE orders (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '订单ID',
    user_id INT NOT NULL COMMENT '用户ID',
    product_id INT NOT NULL COMMENT '产品ID',
    amount DECIMAL(10,2) NOT NULL COMMENT '订单金额',
    status VARCHAR(20) DEFAULT 'PENDING' COMMENT '订单状态: PENDING-待支付, PAID-已支付, CANCELLED-已取消',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_user_id (user_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';

-- 4. 创建产品表
DROP TABLE IF EXISTS products;
CREATE TABLE products (
    id INT PRIMARY KEY AUTO_INCREMENT COMMENT '产品ID',
    name VARCHAR(100) NOT NULL COMMENT '产品名称',
    price DECIMAL(10,2) NOT NULL COMMENT '产品价格',
    stock INT NOT NULL DEFAULT 0 COMMENT '库存数量',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='产品表';

-- 5. 创建 Seata Undo Log 表（AT模式必需）
DROP TABLE IF EXISTS undo_log;
CREATE TABLE undo_log (
    id BIGINT(20) NOT NULL AUTO_INCREMENT COMMENT 'ID',
    branch_id BIGINT(20) NOT NULL COMMENT '分支事务ID',
    xid VARCHAR(100) NOT NULL COMMENT '全局事务ID',
    context VARCHAR(128) NOT NULL COMMENT '上下文',
    rollback_info LONGBLOB NOT NULL COMMENT '回滚信息',
    log_status INT(11) NOT NULL COMMENT '日志状态: 0-正常, 1-已删除',
    log_created DATETIME NOT NULL COMMENT '创建时间',
    log_modified DATETIME NOT NULL COMMENT '修改时间',
    PRIMARY KEY (id),
    UNIQUE KEY ux_undo_log (xid, branch_id)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8 COMMENT='AT模式回滚日志表';

-- 6. 插入测试数据

-- 插入账户数据
INSERT INTO accounts (user_id, balance) VALUES
(1, 10000.00),  -- 用户1，余额10000元
(2, 5000.00);   -- 用户2，余额5000元

-- 插入产品数据
INSERT INTO products (name, price, stock) VALUES
('iPhone 15 Pro', 7999.00, 100),      -- 产品1
('MacBook Pro M3', 12999.00, 50),     -- 产品2
('AirPods Pro', 1999.00, 200),        -- 产品3
('iPad Air', 4799.00, 80);            -- 产品4

-- 7. 验证数据
SELECT '账户表数据：' AS '';
SELECT * FROM accounts;

SELECT '产品表数据：' AS '';
SELECT * FROM products;

SELECT '订单表数据（初始为空）：' AS '';
SELECT * FROM orders;

-- 8. 数据库信息
SELECT 
    '数据库初始化完成' AS '状态',
    DATABASE() AS '当前数据库',
    VERSION() AS 'MySQL版本';

-- =====================================================
-- 初始化完成提示
-- =====================================================
-- 数据库：seata_demo
-- 账户表：accounts (2条记录)
-- 订单表：orders (0条记录)
-- 产品表：products (4条记录)
-- Undo Log表：undo_log (AT模式使用)
-- 
-- 现在可以运行示例程序：
-- go run basic.go       - 基本使用示例
-- go run transfer.go    - 转账示例
-- go run order.go       - 下单示例
-- go run xa_mode.go     - XA模式示例
-- go run fallback.go    - 降级策略示例
-- =====================================================
