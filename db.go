package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"log"
	"os"
)

func initDB() *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		viper.GetString("mysql.user"),
		viper.GetString("mysql.pass"),
		viper.GetString("mysql.addr"),
		viper.GetInt("mysql.port"),
		viper.GetString("mysql.db"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("init db failed,err:%v\n", err)
		os.Exit(1)
	}
	// 尝试与数据库建立连接（校验dsn是否正确）
	err = db.Ping()
	if err != nil {
		log.Printf("ping db failed,err:%v\n", err)
		os.Exit(1)
	}
	return db
}
