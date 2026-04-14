package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/ShiriezH/Gator/internal/config"
	"github.com/ShiriezH/Gator/internal/database"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

// register a command
func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

// run a command
func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

// users handler - lists all users in DB, marks current user
func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	currentUser := s.cfg.CurrentUserName

	for _, user := range users {
		if user.Name == currentUser {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}

	return nil
}

// login handler
func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]

	// check if user exists in DB
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("user does not exist")
	}

	// only set if valid
	if err := s.cfg.SetUser(username); err != nil {
		return err
	}

	fmt.Printf("User set to %s\n", username)
	return nil
}

// register handler
func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("name is required")
	}

	name := cmd.args[0]

	// Check if user already exists
	_, err := s.db.GetUser(context.Background(), name)
	if err == nil {
		return fmt.Errorf("user already exists")
	}

	// Create new user
	user, err := s.db.CreateUser(
		context.Background(),
		database.CreateUserParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Name:      name,
		},
	)
	if err != nil {
		return err
	}

	// Set current user in config
	if err := s.cfg.SetUser(name); err != nil {
		return err
	}

	fmt.Printf("User created: %s\n", user.Name)
	return nil
}

// reset handler - deletes all users from DB
func handlerReset(s *state, cmd command) error {
	err := s.db.DeleteAllUsers(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("Database reset")
	return nil
}

func main() {
	// Load config
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	// Connect to DB
	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)

	s := &state{
		db:  dbQueries,
		cfg: &cfg,
	}

	// Setup commands
	cmds := commands{
		handlers: make(map[string]func(*state, command) error),
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)

	// CLI args
	if len(os.Args) < 2 {
		fmt.Println("not enough arguments")
		os.Exit(1)
	}

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	// Run command
	err = cmds.run(s, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}