package main

import (
	"fmt"
	"os"

	"auth-service/internal/store"
)

func main() {
	dbPath := "/tmp/test_t5.db"
	defer os.Remove(dbPath)

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Printf("FAIL NewSQLiteStore: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Create two users
	u1, _ := s.CreateUser("alice", "hash1", "admin")
	u2, _ := s.CreateUser("bob", "hash2", "viewer")
	fmt.Printf("Created: %s (id=%d), %s (id=%d)\n", u1.Username, u1.ID, u2.Username, u2.ID)

	// ListUsers
	users, err := s.ListUsers("")
	if err != nil {
		fmt.Printf("FAIL ListUsers: %v\n", err)
		os.Exit(1)
	}
	if len(users) != 2 {
		fmt.Printf("FAIL ListUsers: expected 2, got %d\n", len(users))
		os.Exit(1)
	}
	fmt.Printf("PASS: ListUsers returned %d users\n", len(users))

	// empty list
	s2, _ := store.NewSQLiteStore("/tmp/test_t5_empty.db")
	defer os.Remove("/tmp/test_t5_empty.db")
	empty, err := s2.ListUsers("")
	if err != nil {
		fmt.Printf("FAIL ListUsers (empty): %v\n", err)
		os.Exit(1)
	}
	if len(empty) != 0 {
		fmt.Printf("FAIL ListUsers (empty): expected 0, got %d\n", len(empty))
		os.Exit(1)
	}
	fmt.Println("PASS: ListUsers returns empty slice on empty DB")
	s2.Close()

	// UpdateUser
	updated, err := s.UpdateUser(u1.ID, "alice_updated", "newhash", "admin")
	if err != nil {
		fmt.Printf("FAIL UpdateUser: %v\n", err)
		os.Exit(1)
	}
	if updated.Username != "alice_updated" {
		fmt.Printf("FAIL UpdateUser username: expected alice_updated, got %s\n", updated.Username)
		os.Exit(1)
	}
	fmt.Printf("PASS: UpdateUser(%d) → %s\n", updated.ID, updated.Username)

	// UpdateUser duplicate username
	_, err = s.UpdateUser(u1.ID, "bob", "hash", "admin")
	if err == nil {
		fmt.Println("FAIL: duplicate username should error")
		os.Exit(1)
	}
	fmt.Printf("PASS: UpdateUser duplicate error: %v\n", err)

	// UpdateUser not found
	_, err = s.UpdateUser(999, "x", "x", "x")
	if err == nil {
		fmt.Println("FAIL: UpdateUser(999) should error")
		os.Exit(1)
	}
	fmt.Printf("PASS: UpdateUser(999) error: %v\n", err)

	// DeleteUser
	err = s.DeleteUser(u2.ID)
	if err != nil {
		fmt.Printf("FAIL DeleteUser: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("PASS: DeleteUser(%d) succeeded\n", u2.ID)

	// DeleteUser not found
	err = s.DeleteUser(999)
	if err == nil {
		fmt.Println("FAIL: DeleteUser(999) should error")
		os.Exit(1)
	}
	fmt.Printf("PASS: DeleteUser(999) error: %v\n", err)

	// Final list
	users, _ = s.ListUsers("")
	if len(users) != 1 {
		fmt.Printf("FAIL: expected 1 user after delete, got %d\n", len(users))
		os.Exit(1)
	}
	fmt.Printf("PASS: %d user remains after delete\n", len(users))

	fmt.Println("\nALL PASS")
}
