package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

func main() {
	// --- 1. CONFIGURATION ---
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222"
	}

	dbURL := "host=app-db port=5432 user=user password=password dbname=app_data sslmode=disable"

	// --- 2. POSTGRES CONNECTION WITH RETRY LOOP ---
	var db *sql.DB
	var err error

	log.Println("🔌 Main Subscriber: Connecting to Postgres...")
	for i := 0; i < 15; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("⚠️  DB not ready (attempt %d/15): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("❌ DB connection failed: ", err)
	}

	// Create table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ORDERS (
		id SERIAL PRIMARY KEY,
		message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal("❌ Table creation failed: ", err)
	}

	// --- 3. NATS CONNECTION ---
	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("❌ NATS Connection Error: ", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatal("❌ JetStream Error: ", err)
	}

	// Durable Pull Subscription
	sub, err := js.PullSubscribe("orders.new", "MAIN_BATCH_WORKER", nats.BindStream("ORDERS"))
	if err != nil {
		log.Fatal("❌ Subscription Error: ", err)
	}

	log.Println("🚀 Main Subscriber started. Processing in BATCHES of 100...")

	// --- 4. BATCH PROCESSING LOOP ---
	for {
		// Fetch a larger batch of 100 messages
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msgs, err := sub.Fetch(1000, nats.Context(ctx))
		cancel()

		if err != nil {
			if err == nats.ErrTimeout || err == context.DeadlineExceeded {
				continue
			}
			log.Printf("⚠️  Fetch error: %v", err)
			continue
		}

		// Start a SQL Transaction for the batch
		tx, err := db.Begin()
		if err != nil {
			log.Printf("❌ Could not start transaction: %v", err)
			continue
		}

		// Prepare the insert statement once for the whole batch
		stmt, err := tx.Prepare("INSERT INTO ORDERS (message) VALUES ($1)")
		if err != nil {
			log.Printf("❌ Could not prepare statement: %v", err)
			tx.Rollback()
			continue
		}

		successCount := 0
		for _, m := range msgs {
			_, err := stmt.Exec(string(m.Data))
			if err != nil {
				log.Printf("❌ Failed to insert message in batch: %v", err)
				continue
			}
			successCount++
		}

		// Commit the transaction to disk
		err = tx.Commit()
		if err != nil {
			log.Printf("❌ Failed to commit transaction: %v", err)
			// NAK all messages so they are redelivered if commit fails
			for _, m := range msgs {
				m.Nak()
			}
		} else {
			// ACK all messages in the batch only after successful commit
			for _, m := range msgs {
				m.Ack()
			}
			log.Printf("📦 Batch processed: %d orders saved to Postgres", successCount)
		}
		stmt.Close()
	}
}