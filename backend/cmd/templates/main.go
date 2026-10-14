package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/soren/backend/internal/templateserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "credential" {
		if len(os.Args) != 4 {
			log.Fatal("用法：soren-templates credential <发布者ID> <显示名称>")
		}
		raw := make([]byte, 32)
		if _, e := rand.Read(raw); e != nil {
			log.Fatal(e)
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		publisher := templateserver.Publisher{ID: os.Args[2], Name: os.Args[3], TokenHash: fmt.Sprintf("%x", sha256.Sum256([]byte(token)))}
		output, _ := json.MarshalIndent(publisher, "", "  ")
		fmt.Printf("将以下对象加入 publishers.json 数组：\n%s\n\n发布凭据（仅交给该发布者）：\n%s\n", output, token)
		return
	}
	origin := os.Getenv("TEMPLATE_PUBLIC_URL")
	usersPath := os.Getenv("TEMPLATE_PUBLISHERS_FILE")
	if usersPath == "" {
		usersPath = "publishers.json"
	}
	raw, e := os.ReadFile(usersPath)
	if e != nil {
		log.Fatal("读取发布者配置失败：", e)
	}
	var publishers []templateserver.Publisher
	if json.Unmarshal(raw, &publishers) != nil {
		log.Fatal("发布者配置需为 JSON 数组")
	}
	dbPath := os.Getenv("TEMPLATE_DATABASE")
	if dbPath == "" {
		dbPath = "data/templates.db"
	}
	if e = os.MkdirAll(filepath.Dir(dbPath), 0700); e != nil {
		log.Fatal(e)
	}
	db, e := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		log.Fatal(e)
	}
	conn, e := db.DB()
	if e != nil {
		log.Fatal(e)
	}
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	if e = os.Chmod(dbPath, 0600); e != nil {
		log.Fatal(e)
	}
	service, e := templateserver.New(db, origin, publishers)
	if e != nil {
		log.Fatal(e)
	}
	addr := os.Getenv("TEMPLATE_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	server := &http.Server{Addr: addr, Handler: service.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Printf("Soren 模板服务已启动：%s", addr)
	log.Fatal(server.ListenAndServe())
}
