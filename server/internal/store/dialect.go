package store

import (
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func mysqlOpen(dsn string) gorm.Dialector {
	return gmysql.Open(dsn)
}
