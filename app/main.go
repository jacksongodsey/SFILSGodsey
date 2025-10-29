package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xuri/excelize/v2"
)

const (
	user   = "root"
	host   = "127.0.0.1"
	port   = "3306"
	dbName = "sfils"
)

func main() {
	// environment variable grabbing for the password.
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		// hardcoded password is below but not recommended for production.
		password = ""
		log.Println("warning. using unsecured hardcoded password.")
	}

	// connecting to mysql.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("couldn't open the database:", err)
	}
	defer db.Close()

	// setting conection settings - not sure if these numbers are optimal but they work
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("couldn't connect to db:", err)
	}

	// create the database if it doesn't exist.
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + dbName)
	if err != nil {
		log.Fatal("couldn't create the database:", err)
	}
	fmt.Println("database", dbName, "ready.")

	// switch to the new database
	_, err = db.Exec("USE " + dbName)
	if err != nil {
		log.Fatal("couldn't switch to database:", err)
	}

	// running all the scripts in the scripts folder.
	err = runScripts(db, "../scripts")
	if err != nil {
		log.Fatal(err)
	}

	// read excel and import data.
	err = importExcel(db, "../data/sfpl.xlsx")
	if err != nil {
		log.Fatal(err)
	}

	// start the text interface
	startTextInterface(db)
}

// running all the scripts in the folder
func runScripts(db *sql.DB, folder string) error {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) == ".sql" {
			filePath := filepath.Join(folder, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			// we are splitting by semicolons to ensure we can handle multiple statements.
			// learned this the hard way when DROP TABLE and CREATE TABLE weren't working together
			statements := strings.Split(string(content), ";")

			for _, stmt := range statements {
				// removing any comments - had issues with comments breaking things
				lines := strings.Split(stmt, "\n")
				var cleanLines []string
				for _, line := range lines {
					if idx := strings.Index(line, "--"); idx >= 0 {
						line = line[:idx]
					}
					line = strings.TrimSpace(line)
					if line != "" {
						cleanLines = append(cleanLines, line)
					}
				}

				stmt = strings.Join(cleanLines, " ")
				stmt = strings.TrimSpace(stmt)

				if stmt == "" {
					continue // skipping any possible empty statements.
				}

				_, err = db.Exec(stmt)
				if err != nil {
					return fmt.Errorf("error executing statement in %s: %v\nstatement: %s", entry.Name(), err, stmt)
				}
			}

			fmt.Println("executed script:", entry.Name())
		}
	}
	return nil
}

// converting month names to integers or returning null
func monthToIntOrNull(monthName string) interface{} {
	monthName = strings.TrimSpace(monthName)

	// empty values we return null
	if monthName == "" {
		return nil
	}

	// probably could have done this more elegantly but a map works fine
	months := map[string]int{
		"january": 1, "february": 2, "march": 3, "april": 4,
		"may": 5, "june": 6, "july": 7, "august": 8,
		"september": 9, "october": 10, "november": 11, "december": 12,
	}

	monthLower := strings.ToLower(monthName)
	if num, ok := months[monthLower]; ok {
		return num
	}

	// if it's not empty but we can't understand it we print and then return null.
	fmt.Printf("warning: can't recognize month name: '%s'\n", monthName)
	return nil
}

// converting a string to an int or returns nil if the values are empty
func stringToIntOrNull(value string) interface{} {
	value = strings.TrimSpace(value)

	// returning null for an empty value
	if value == "" {
		return nil
	}

	return value
}

// validates and cleans the email address
// excel data has different values for email address that we need to filter for
func cleanEmail(email string) interface{} {
	email = strings.TrimSpace(email)

	// return null if empty, true/false, or anything invalid.
	if email == "" ||
		strings.EqualFold(email, "true") ||
		strings.EqualFold(email, "false") ||
		!strings.Contains(email, "@") {
		return nil
	}

	return email
}

// get or insert patron type and return its ID
func getPatronTypeID(tx *sql.Tx, code, desc string) (int, error) {
	var id int
	err := tx.QueryRow("SELECT id FROM patron_types WHERE code = ?", code).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := tx.Exec("INSERT INTO patron_types (code, description) VALUES (?, ?)", code, desc)
		if err != nil {
			return 0, err
		}
		insertID, _ := res.LastInsertId()
		id = int(insertID)
	} else if err != nil {
		return 0, err
	}
	return id, nil
}

// ensure library exists
func ensureLibrary(tx *sql.Tx, code, name string) error {
	_, err := tx.Exec("INSERT IGNORE INTO libraries (code, name) VALUES (?, ?)", code, name)
	return err
}

// ensure notification type exists
func ensureNotificationType(tx *sql.Tx, code, name string) error {
	_, err := tx.Exec("INSERT IGNORE INTO notification_types (code, description) VALUES (?, ?)", code, name)
	return err
}

// reads an excel file and puts the data into the patrons table
func importExcel(db *sql.DB, file string) error {
	f, err := excelize.OpenFile(file)
	if err != nil {
		return err
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return err
	}

	// using a transaction so if something fails we can rollback
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// TODO: not 100% sure this is the right way to handle errors but it seems to work
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// prepared statement. using the same statement is quicker
	stmt, err := tx.Prepare(`
		INSERT INTO patrons (
			patron_type_id, checkout_total, renewal_total,
			age_range, home_library_code, active_month, active_year,
			notification_type_code, email, within_sfc, year_registered
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	good := 0
	bad := 0

	for i, row := range rows {
		if i == 0 { // skip the header here to make sure this all works.
			continue
		}

		// make sure we have enough columns
		if len(row) < 14 {
			fmt.Printf("skipping row %d: insufficient columns (has %d, needs 14)\n", i, len(row))
			bad++
			continue
		}

		// clean up each cell
		for j := range row {
			row[j] = strings.TrimSpace(strings.ReplaceAll(row[j], "\n", " "))
		}

		// create type and get the id
		patronTypeID, err := getPatronTypeID(tx, row[0], row[1])
		if err != nil {
			fmt.Printf("failed patron type for row %d: %v\n", i, err)
			bad++
			continue
		}

		// ensure library exists
		if err := ensureLibrary(tx, row[5], row[6]); err != nil {
			fmt.Printf("failed library for row %d: %v\n", i, err)
			bad++
			continue
		}

		// ensure notification type exists
		if err := ensureNotificationType(tx, row[9], row[10]); err != nil {
			fmt.Printf("failed notification for row %d: %v\n", i, err)
			bad++
			continue
		}

		// convert the month to number or a null value
		activeMonth := monthToIntOrNull(row[7])

		// convert the year to number or a null value
		activeYear := stringToIntOrNull(row[8])
		yearRegistered := stringToIntOrNull(row[13])

		// converts bools from true/false to 1/0
		withinSFC := 0
		if strings.EqualFold(row[12], "true") {
			withinSFC = 1
		}

		// clean email and set to null if it's not in a valid format
		email := cleanEmail(row[11])

		// insert data with patron_type_id instead of code/def
		_, err = stmt.Exec(
			patronTypeID, row[2], row[3],
			row[4], row[5], activeMonth, activeYear,
			row[9], email, withinSFC, yearRegistered,
		)
		if err != nil {
			fmt.Printf("failed to insert row %d: %v\n", i, err)
			if bad < 5 { // Only show the first 5 if we get an error
				fmt.Printf("row data: %v\n", row)
			}
			bad++
			continue
		}

		good++
		// print progress every 10k rows so i know it's working
		if i%10000 == 0 {
			fmt.Printf("processed %d rows (%d successful, %d errors)\n", i, good, bad)
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("\nexcel import complete:\n")
	fmt.Printf("  total rows processed: %d\n", len(rows)-1)
	fmt.Printf("  successful inserts: %d\n", good)
	fmt.Printf("  failed inserts: %d\n", bad)

	return nil
}

// providing a very basic text interface
func startTextInterface(db *sql.DB) {
	fmt.Println("\n=== Program interface ===")
	fmt.Println("Type SQL queries to run (best to run select queries)")
	fmt.Println("Type 'exit' or 'quit' to quit")
	fmt.Println("Type 'help' for example queries")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("See ya!")
			break
		}

		if input == "help" {
			printHelp()
			continue
		}
		if input == "benchmark" {
			runBenchmark(db)
			continue
		}

		// run the query
		rows, err := db.Query(input)
		if err != nil {
			fmt.Println("query error:", err)
			continue
		}

		// get the names of columns
		cols, err := rows.Columns()
		if err != nil {
			fmt.Println("error getting columns:", err)
			rows.Close()
			continue
		}

		// print headers of the columns
		fmt.Println(strings.Repeat("-", 80))
		for i, col := range cols {
			if i > 0 {
				fmt.Print(" | ")
			}
			fmt.Print(col)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("-", 80))

		// making containers for the values. handy go feature
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// printing the rows
		rowCount := 0
		for rows.Next() {
			err := rows.Scan(valuePtrs...)
			if err != nil {
				fmt.Println("error scanning row:", err)
				continue
			}

			for i, val := range values {
				if i > 0 {
					fmt.Print(" | ")
				}

				// null handling
				if val == nil {
					fmt.Print("NULL")
				} else {
					// converting the byte arrays to strings
					switch v := val.(type) {
					case []byte:
						fmt.Print(string(v))
					default:
						fmt.Print(v)
					}
				}
			}
			fmt.Println()
			rowCount++
		}

		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%d rows returned\n\n", rowCount)
		rows.Close()
	}
}

// some example queries
func printHelp() {
	fmt.Println("\n=== Some example queries you can try ===")
	fmt.Println("SELECT COUNT(*) FROM patrons;")
	fmt.Println("SELECT * FROM patrons LIMIT 10;")
	fmt.Println("SELECT patron_type_def, COUNT(*) as count FROM patrons GROUP BY patron_type_def;")
	fmt.Println("SELECT age_range, COUNT(*) as count FROM patrons GROUP BY age_range ORDER BY count DESC;")
	fmt.Println("SELECT home_library_def, COUNT(*) as count FROM patrons WHERE within_sfc = 1 GROUP BY home_library_def;")
	fmt.Println("SELECT * FROM patrons WHERE email LIKE '%@gmail.com%' LIMIT 5;")
	fmt.Println("\nType 'benchmark' to run performance tests")
	fmt.Println()
}

// benchmark to test performance
func runBenchmark(db *sql.DB) {
	fmt.Println("\n=== performance test ===")

	tests := []struct {
		name  string
		query string
	}{
		{"Count all patrons", "SELECT COUNT(*) FROM patrons"},
		{"Count by patron type", "SELECT pt.description, COUNT(*) FROM patrons p JOIN patron_types pt ON p.patron_type_id = pt.id GROUP BY pt.description"},
		{"Count by age range", "SELECT age_range, COUNT(*) FROM patrons GROUP BY age_range"},
		{"Count by library", "SELECT l.name, COUNT(*) FROM patrons p JOIN libraries l ON p.home_library_code = l.code GROUP BY l.name"},
		{"Find SF patrons", "SELECT COUNT(*) FROM patrons WHERE within_sfc = 1"},
		{"Active in 2023", "SELECT COUNT(*) FROM patrons WHERE active_year = 2023"},
	}

	for _, test := range tests {
		start := time.Now()
		rows, err := db.Query(test.query)
		if err != nil {
			fmt.Printf("%s: error - %v\n", test.name, err)
			continue
		}

		// count results
		count := 0
		for rows.Next() {
			count++
		}
		rows.Close()

		elapsed := time.Since(start)
		fmt.Printf("✓ %s: %v (%d rows)\n", test.name, elapsed, count)
	}

	fmt.Println("\nbenchmark done")
}
