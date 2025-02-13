package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

type User struct {
	ID    int
	Name  string
	Email string
	Age   int
}

func initializeDB() (*sql.DB, error) {
	// Connect as root to create the database and user
	rootDsn := "root:@tcp(127.0.0.1:3306)/"
	db, err := sql.Open("mysql", rootDsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Create the database if it doesn't exist
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS go_db")
	if err != nil {
		return nil, fmt.Errorf("error creating database: %w", err)
	}

	// Create a new user if it doesn't exist
	newUser := "go_user"
	newPassword := "password"
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'", newUser, newPassword))
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Grant all privileges on go_db to the new user
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON go_db.* TO '%s'@'localhost'", newUser))
	if err != nil {
		return nil, fmt.Errorf("error granting privileges: %w", err)
	}

	// Apply the changes in privileges
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return nil, fmt.Errorf("error flushing privileges: %w", err)
	}

	fmt.Println("Database and user successfully created!")

	// Connect using the new user
	userDsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/go_db", newUser, newPassword)
	db, err = sql.Open("mysql", userDsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting with new user: %w", err)
	}

	// Create the User table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS User (
		ID INT PRIMARY KEY AUTO_INCREMENT,
		Name VARCHAR(100) NOT NULL,
		Email VARCHAR(100) UNIQUE NOT NULL,
		Age INT
	);`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	// Insert test data if the table has no records
	_, err = db.Exec("INSERT INTO User (Name, Email, Age) SELECT * FROM (SELECT 'Amna Jusić', 'ajusic5@etf.unsa.ba', 25) AS tmp WHERE NOT EXISTS (SELECT * FROM User)")
	if err != nil {
		return nil, fmt.Errorf("error inserting data: %w", err)
	}

	return db, nil
}

func main() {
	db, err := initializeDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Retrieve all users to test the database
	rows, err := db.Query("SELECT ID, Name, Email, Age FROM User")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Iterate through the results
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age); err != nil {
			log.Fatal(err)
		}
		users = append(users, u)
	}

	// Check for errors after iteration
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Print users
	fmt.Println("User list:")
	for _, user := range users {
		fmt.Printf("ID: %d, Name: %s, Email: %s, Age: %d\n", user.ID, user.Name, user.Email, user.Age)
	}
}
