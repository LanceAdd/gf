// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
)

// buildDSN 构建 MySQL DSN 连接字符串（仅用于测试）
// 注意：新方案中实际使用 mysql.Driver 的 Open 方法，不需要手动构建 DSN
// 这个函数仅用于兼容旧测试用例
func buildDSN(node *gdb.ConfigNode) string {
	// user:password@tcp(host:port)/dbname?charset=utf8mb4&loc=timezone
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		node.User,
		node.Pass,
		node.Host,
		node.Port,
		node.Name,
	)

	// 添加 charset
	if node.Charset != "" {
		dsn += "?charset=" + node.Charset
	} else {
		dsn += "?charset=utf8mb4"
	}

	// 添加 timezone
	if node.Timezone != "" {
		dsn += "&loc=" + node.Timezone
	}

	// 添加额外参数
	if node.Extra != "" {
		dsn += "&" + node.Extra
	}

	return dsn
}
