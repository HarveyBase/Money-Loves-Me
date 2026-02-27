package main

import (
	"fmt"
	"log"
	"os"

	"money-loves-me/internal/config"
	"money-loves-me/internal/model"
	"money-loves-me/internal/server"

	"gorm.io/gorm"
)

// seed 工具：创建测试用户
func main() {
	configPath := "configs/config.dev.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := model.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 创建默认管理员用户
	createUser(db, "admin", "admin123")
	fmt.Println("测试用户创建完成: admin / admin123")
}

func createUser(db *gorm.DB, username, password string) {
	hash, err := server.HashPassword(password)
	if err != nil {
		log.Fatalf("密码哈希失败: %v", err)
	}

	user := model.User{
		Username:     username,
		PasswordHash: hash,
	}

	result := db.Where("username = ?", username).FirstOrCreate(&user)
	if result.Error != nil {
		log.Fatalf("创建用户失败: %v", result.Error)
	}
}
