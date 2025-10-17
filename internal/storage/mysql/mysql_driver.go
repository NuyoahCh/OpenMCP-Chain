//go:build mysql

package mysql

import (
	_ "github.com/go-sql-driver/mysql"
)

// 该文件在构建 tag 为 mysql 时被编译，负责注册 MySQL 驱动。
