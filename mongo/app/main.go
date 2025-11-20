package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoURI = "mongodb://localhost:27017"
	dbName   = "sfils"
)

// PatronType represents patron type documents
type PatronType struct {
	Code        string `bson:"code"`
	Description string `bson:"description"`
}

// Library represents library documents
type Library struct {
	Code string `bson:"code"`
	Name string `bson:"name"`
}

// NotificationType represents notification type documents
type NotificationType struct {
	Code        string `bson:"code"`
	Description string `bson:"description"`
}

// Patron represents patron documents
type Patron struct {
	PatronTypeCode       string  `bson:"patron_type_code"`
	PatronTypeDesc       string  `bson:"patron_type_desc"`
	CheckoutTotal        string  `bson:"checkout_total"`
	RenewalTotal         string  `bson:"renewal_total"`
	AgeRange             string  `bson:"age_range"`
	HomeLibraryCode      string  `bson:"home_library_code"`
	HomeLibraryName      string  `bson:"home_library_name"`
	ActiveMonth          *int    `bson:"active_month,omitempty"`
	ActiveYear           *string `bson:"active_year,omitempty"`
	NotificationTypeCode string  `bson:"notification_type_code"`
	NotificationTypeDesc string  `bson:"notification_type_desc"`
	Email                *string `bson:"email,omitempty"`
	WithinSFC            bool    `bson:"within_sfc"`
	YearRegistered       *string `bson:"year_registered,omitempty"`
}

func main() {
	// environment variable grabbing for the connection string
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		// hardcoded URI is below but not recommended for production
		uri = mongoURI
		log.Println("warning: using default mongodb connection string")
	}

	// connecting to mongodb
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal("couldn't connect to mongodb:", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Fatal("error disconnecting:", err)
		}
	}()

	// ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("couldn't ping mongodb:", err)
	}

	db := client.Database(dbName)
	fmt.Println("database", dbName, "ready")

	// create indexes for better query performance
	err = createIndexes(db)
	if err != nil {
		log.Fatal(err)
	}

	// read excel and import data
	err = importExcel(db, "../data/sfpl.xlsx")
	if err != nil {
		log.Fatal(err)
	}

	// start the text interface
	startTextInterface(db)
}

// create indexes on collections for better performance
func createIndexes(db *mongo.Database) error {
	ctx := context.Background()

	// index on patron_types code field (unique)
	_, err := db.Collection("patron_types").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("error creating patron_types index: %v", err)
	}

	// index on libraries code field (unique)
	_, err = db.Collection("libraries").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("error creating libraries index: %v", err)
	}

	// index on notification_types code field (unique)
	_, err = db.Collection("notification_types").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("error creating notification_types index: %v", err)
	}

	// indexes on patrons collection for common queries
	patronIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "patron_type_code", Value: 1}}},
		{Keys: bson.D{{Key: "age_range", Value: 1}}},
		{Keys: bson.D{{Key: "home_library_code", Value: 1}}},
		{Keys: bson.D{{Key: "within_sfc", Value: 1}}},
		{Keys: bson.D{{Key: "active_year", Value: 1}}},
		{Keys: bson.D{{Key: "email", Value: 1}}},
	}

	_, err = db.Collection("patrons").Indexes().CreateMany(ctx, patronIndexes)
	if err != nil {
		return fmt.Errorf("error creating patrons indexes: %v", err)
	}

	fmt.Println("indexes created successfully")
	return nil
}

// converting month names to integers or returning nil
func monthToIntOrNull(monthName string) *int {
	monthName = strings.TrimSpace(monthName)

	// empty values we return nil
	if monthName == "" {
		return nil
	}

	// map
	months := map[string]int{
		"january": 1, "february": 2, "march": 3, "april": 4,
		"may": 5, "june": 6, "july": 7, "august": 8,
		"september": 9, "october": 10, "november": 11, "december": 12,
	}

	monthLower := strings.ToLower(monthName)
	if num, ok := months[monthLower]; ok {
		return &num
	}

	// if it's not empty but we can't understand it we print and then return nil
	fmt.Printf("warning: can't recognize month name: '%s'\n", monthName)
	return nil
}

// converting a string to a pointer or returns nil if the value is empty
func stringToPointerOrNull(value string) *string {
	value = strings.TrimSpace(value)

	// returning nil for an empty value
	if value == "" {
		return nil
	}

	return &value
}

// validates and cleans the email address
// excel data has different values for email address that we need to filter for
func cleanEmail(email string) *string {
	email = strings.TrimSpace(email)

	// return nil if empty, true/false, or anything invalid
	if email == "" ||
		strings.EqualFold(email, "true") ||
		strings.EqualFold(email, "false") ||
		!strings.Contains(email, "@") {
		return nil
	}

	return &email
}

// ensure patron type exists in the collection
func ensurePatronType(ctx context.Context, coll *mongo.Collection, code, desc string) error {
	filter := bson.M{"code": code}
	update := bson.M{"$setOnInsert": bson.M{"code": code, "description": desc}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// ensure library exists in the collection
func ensureLibrary(ctx context.Context, coll *mongo.Collection, code, name string) error {
	filter := bson.M{"code": code}
	update := bson.M{"$setOnInsert": bson.M{"code": code, "name": name}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// ensure notification type exists in the collection
func ensureNotificationType(ctx context.Context, coll *mongo.Collection, code, desc string) error {
	filter := bson.M{"code": code}
	update := bson.M{"$setOnInsert": bson.M{"code": code, "description": desc}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// reads an excel file and puts the data into the patrons collection
func importExcel(db *mongo.Database, file string) error {
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

	ctx := context.Background()

	// drop existing data for a fresh start
	if err := db.Collection("patrons").Drop(ctx); err != nil {
		fmt.Println("note: couldn't drop patrons collection")
	}
	if err := db.Collection("patron_types").Drop(ctx); err != nil {
		fmt.Println("note: couldn't drop patron_types collection")
	}
	if err := db.Collection("libraries").Drop(ctx); err != nil {
		fmt.Println("note: couldn't drop libraries collection")
	}
	if err := db.Collection("notification_types").Drop(ctx); err != nil {
		fmt.Println("note: couldn't drop notification_types collection")
	}

	// recreate indexes after dropping
	if err := createIndexes(db); err != nil {
		return err
	}

	patronsColl := db.Collection("patrons")
	patronTypesColl := db.Collection("patron_types")
	librariesColl := db.Collection("libraries")
	notificationTypesColl := db.Collection("notification_types")

	good := 0
	bad := 0

	// using bulk writes for better performance
	var patronDocs []interface{}

	for i, row := range rows {
		if i == 0 { // skip the header here to make sure this all works
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

		// ensure patron type exists
		if err := ensurePatronType(ctx, patronTypesColl, row[0], row[1]); err != nil {
			fmt.Printf("failed patron type for row %d: %v\n", i, err)
			bad++
			continue
		}

		// ensure library exists
		if err := ensureLibrary(ctx, librariesColl, row[5], row[6]); err != nil {
			fmt.Printf("failed library for row %d: %v\n", i, err)
			bad++
			continue
		}

		// ensure notification type exists
		if err := ensureNotificationType(ctx, notificationTypesColl, row[9], row[10]); err != nil {
			fmt.Printf("failed notification for row %d: %v\n", i, err)
			bad++
			continue
		}

		// convert the month to number or a nil value
		activeMonth := monthToIntOrNull(row[7])

		// convert the year to pointer or a nil value
		activeYear := stringToPointerOrNull(row[8])
		yearRegistered := stringToPointerOrNull(row[13])

		// converts bools from true/false to boolean
		withinSFC := strings.EqualFold(row[12], "true")

		// clean email and set to nil if it's not in a valid format
		email := cleanEmail(row[11])

		// create patron document with embedded reference data
		patron := Patron{
			PatronTypeCode:       row[0],
			PatronTypeDesc:       row[1],
			CheckoutTotal:        row[2],
			RenewalTotal:         row[3],
			AgeRange:             row[4],
			HomeLibraryCode:      row[5],
			HomeLibraryName:      row[6],
			ActiveMonth:          activeMonth,
			ActiveYear:           activeYear,
			NotificationTypeCode: row[9],
			NotificationTypeDesc: row[10],
			Email:                email,
			WithinSFC:            withinSFC,
			YearRegistered:       yearRegistered,
		}

		patronDocs = append(patronDocs, patron)
		good++

		// batch insert every 1000 documents for better performance
		if len(patronDocs) >= 1000 {
			_, err := patronsColl.InsertMany(ctx, patronDocs)
			if err != nil {
				fmt.Printf("failed to insert batch at row %d: %v\n", i, err)
				bad += len(patronDocs)
				good -= len(patronDocs)
			}
			patronDocs = []interface{}{}
		}

		// print progress every 10k rows so i know it's working
		if i%10000 == 0 {
			fmt.Printf("processed %d rows (%d successful, %d errors)\n", i, good, bad)
		}
	}

	// insert remaining documents
	if len(patronDocs) > 0 {
		_, err := patronsColl.InsertMany(ctx, patronDocs)
		if err != nil {
			fmt.Printf("failed to insert final batch: %v\n", err)
			bad += len(patronDocs)
			good -= len(patronDocs)
		}
	}

	fmt.Printf("\nexcel import complete:\n")
	fmt.Printf("  total rows processed: %d\n", len(rows)-1)
	fmt.Printf("  successful inserts: %d\n", good)
	fmt.Printf("  failed inserts: %d\n", bad)

	return nil
}

// providing a very basic text interface
func startTextInterface(db *mongo.Database) {
	fmt.Println("\n=== program interface ===")
	fmt.Println("type MongoDB queries in JSON format")
	fmt.Println("format: collection_name|{\"field\": \"value\"}")
	fmt.Println("type 'exit' or 'quit' to quit")
	fmt.Println("type 'help' for example queries")
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
			fmt.Println("bye")
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

		// parse command format: collection|filter
		parts := strings.SplitN(input, "|", 2)
		if len(parts) != 2 {
			fmt.Println("format error: use collection_name|{filter}")
			continue
		}

		collectionName := strings.TrimSpace(parts[0])
		filterStr := strings.TrimSpace(parts[1])

		// parse the filter as BSON
		var filter bson.M
		if filterStr != "{}" && filterStr != "" {
			err := bson.UnmarshalExtJSON([]byte(filterStr), true, &filter)
			if err != nil {
				fmt.Println("filter parse error:", err)
				continue
			}
		}

		// execute the query
		ctx := context.Background()
		cursor, err := db.Collection(collectionName).Find(ctx, filter, options.Find().SetLimit(100))
		if err != nil {
			fmt.Println("query error:", err)
			continue
		}

		// decode and print results
		fmt.Println(strings.Repeat("-", 80))
		rowCount := 0
		for cursor.Next(ctx) {
			var result bson.M
			if err := cursor.Decode(&result); err != nil {
				fmt.Println("decode error:", err)
				continue
			}
			fmt.Printf("%v\n", result)
			rowCount++
		}

		cursor.Close(ctx)
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%d documents returned\n\n", rowCount)
	}
}

// some example queries
func printHelp() {
	fmt.Println("\n=== Some example queries you can try ===")
	fmt.Println("patrons|{}  // Get first 100 patrons")
	fmt.Println("patrons|{\"within_sfc\": true}  // Find SF patrons")
	fmt.Println("patrons|{\"age_range\": \"25 to 34 years\"}  // Find patrons by age")
	fmt.Println("patrons|{\"email\": {\"$regex\": \"gmail.com\"}}  // Find gmail users")
	fmt.Println("patron_types|{}  // List all patron types")
	fmt.Println("libraries|{}  // List all libraries")
	fmt.Println("\nType 'benchmark' to run performance tests")
	fmt.Println()
}

// benchmark to test performance
func runBenchmark(db *mongo.Database) {
	fmt.Println("\n=== performance test ===")
	ctx := context.Background()

	tests := []struct {
		name       string
		collection string
		pipeline   interface{}
	}{
		{
			"count all patrons",
			"patrons",
			bson.M{},
		},
		{
			"count by patron type",
			"patrons",
			mongo.Pipeline{
				{{Key: "$group", Value: bson.M{
					"_id":   "$patron_type_desc",
					"count": bson.M{"$sum": 1},
				}}},
			},
		},
		{
			"count by age range",
			"patrons",
			mongo.Pipeline{
				{{Key: "$group", Value: bson.M{
					"_id":   "$age_range",
					"count": bson.M{"$sum": 1},
				}}},
			},
		},
		{
			"count by library",
			"patrons",
			mongo.Pipeline{
				{{Key: "$group", Value: bson.M{
					"_id":   "$home_library_name",
					"count": bson.M{"$sum": 1},
				}}},
			},
		},
		{
			"find SF patrons",
			"patrons",
			bson.M{"within_sfc": true},
		},
		{
			"active in 2023",
			"patrons",
			bson.M{"active_year": "2023"},
		},
	}

	for _, test := range tests {
		start := time.Now()

		var count int
		switch p := test.pipeline.(type) {
		case mongo.Pipeline:
			cursor, err := db.Collection(test.collection).Aggregate(ctx, p)
			if err != nil {
				fmt.Printf("%s: error - %v\n", test.name, err)
				continue
			}
			for cursor.Next(ctx) {
				count++
			}
			cursor.Close(ctx)
		case bson.M:
			c, err := db.Collection(test.collection).CountDocuments(ctx, p)
			if err != nil {
				fmt.Printf("%s: error - %v\n", test.name, err)
				continue
			}
			count = int(c)
		}

		elapsed := time.Since(start)
		fmt.Printf("%s: %v (%d results)\n", test.name, elapsed, count)
	}

	fmt.Println("\nbenchmark done")
}
