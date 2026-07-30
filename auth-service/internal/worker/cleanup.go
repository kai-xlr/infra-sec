package worker

import (
	"log"
	"time"

	"auth-service/internal/store"
)

type CleanupWorker struct {
	store    store.Store
	interval time.Duration
	done     chan struct{}
}

func NewCleanupWorker(s store.Store, interval time.Duration, done chan struct{}) *CleanupWorker {
	return &CleanupWorker{
		store:    s,
		interval: interval,
		done:     done,
	}
}

func (w *CleanupWorker) Start() {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				users, err := w.store.ListUsers("")
				if err != nil {
					log.Printf("Failed to retrieve users: %v", err)
					continue
				}

				for _, user := range users {
					if time.Since(user.UpdatedAt) > 24*time.Hour {
						log.Printf("Stale user identified: ID=%d, UpdatedAt=%v", user.ID, user.UpdatedAt)
					}
				}

			case <-w.done:
				log.Println("cleanup worker stopped")
				return
			}
		}
	}()
}
