package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/enterprise"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

func main() {
	models := config.Load()
	c, err := enterprise.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	db, err := store.OpenDB(&config.Config{StorageDriver: config.StorageDriverPostgres, DatabaseURL: c.DatabaseURL})
	if err != nil {
		log.Fatal("Cannot open enterprise database")
	}
	sql, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sql.Close()
	sql.SetMaxOpenConns(10)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "users":
			var users []enterprise.User
			if db.Order("created_at").Find(&users).Error != nil {
				log.Fatal("Cannot list users")
			}
			_ = json.NewEncoder(os.Stdout).Encode(users)
		case "disable", "enable":
			if len(os.Args) != 3 {
				log.Fatal("Usage: soren-enterprise disable|enable USER_ID")
			}
			// Changing membership and revoking sessions must succeed together.
			if err := db.Transaction(func(tx *gorm.DB) error {
				r := tx.Model(&enterprise.User{}).Where("id = ?", os.Args[2]).Update("disabled", os.Args[1] == "disable")
				if r.Error != nil || r.RowsAffected != 1 {
					return errors.New("user not found or update failed")
				}
				return tx.Exec("DELETE FROM enterprise_sessions WHERE user_id = ?", os.Args[2]).Error
			}); err != nil {
				log.Fatal("Cannot change membership and revoke sessions")
			}
		default:
			log.Fatal("Usage: soren-enterprise [users|disable USER_ID|enable USER_ID]")
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Fail closed: the existing runtime's recovery is only safe with one instance.
	lock, err := sql.Conn(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer lock.Close()
	var owned bool
	if err = lock.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(73667301)").Scan(&owned); err != nil || !owned {
		log.Fatal("Another enterprise server is running; v1 requires one instance per database")
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, cancel := context.WithTimeout(ctx, 3*time.Second)
				err := lock.PingContext(check)
				cancel()
				if err != nil {
					log.Fatal("Database lease lost; stopping to prevent concurrent execution")
				}
			}
		}
	}()
	var p llm.Provider
	name, model := "local-echo", models.Model
	switch {
	case models.AnthropicAPIKey != "":
		p = llm.NewAnthropicProvider(models.AnthropicAPIKey, models.AnthropicBaseURL)
		name = "anthropic"
	case models.OpenAIAPIKey != "":
		p = llm.NewOpenAIProvider(models.OpenAIAPIKey, models.OpenAIBaseURL)
		name = "openai-compatible"
	case c.Development:
		p = llm.NewMockProvider()
	default:
		log.Fatal("Configure an enterprise model API key before starting")
	}
	if model == "" {
		if !c.Development {
			log.Fatal("Configure SOREN_MODEL before starting")
		}
		model = "demo"
	}
	ws := enterprise.NewWorkspaces(c, db, p, model, name)
	s, err := enterprise.New(c, db, ws.Handler)
	if err != nil {
		log.Fatal(err)
	}
	ws.Active = s.Active
	server := &http.Server{Addr: c.Listen, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Soren enterprise listening on %s (%s); single-instance pilot", c.Listen, c.Name)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
