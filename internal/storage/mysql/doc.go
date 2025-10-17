// Package mysql 提供 MySQL 风格的仓储接口，既包含基于本地文件的内存实现，
// 也在启用 `mysql` build tag 时提供真正的 SQLTaskRepository，便于在开发与生产
// 场景之间无缝切换。
package mysql
