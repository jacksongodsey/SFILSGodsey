# SFILS - San Francisco Library Patron Database

This is a go program that takes the San Francisco library patron data in an excel format and converts it into a MySQL database which you can query.
## What It Does

- Reads data from an Excel file
- Creates and imports a MySQL database
- Provides a very basic command lind interface to run queries with SQL.
- Aims to handle data that isn't extremely coherent. 

## Setup

### Requirements
- Go installation
- MySQL install and running locally
- The patron Excel file (`sfpl.xlsx`)

### Installation
- This program assumes that you are using the root account on your local machine for the database on the default port. You can export your password in your shell as seen below or hardcode it and other information in the code.

```bash
# Install required Go packages.
go get github.com/go-sql-driver/mysql
go get github.com/xuri/excelize/v2

# Set your MySQL password
export DB_PASSWORD="your_password"

# Run the program
cd app
go run main.go
```

## Structure of the Project

```
project/
├── app/
│   └── main.go           # Main program
├── scripts/
│   └── create_tables.sql # Script to create the db schema
└── data/
    └── sfpl.xlsx         # Patron Excel file
```

## How It Works

1. Connects to MySQL using credentials provided in the code or via a shell variable
2. Creates database called `sfils` if it does not currently exist
3. Runs SQL scripts from the `scripts/` folder to create tables
4. Imports Excel data and cleans it up and inserts the data into the database
5. Opens query interface where you can run SQL commands on the data

## Database Schema

The `patrons` table has several columns which can be seen below:
- Standard info: patron type, age range
- Library: home library location
- Activity: checkouts, renewals, active month/year
- Other: email, within SF county, registration year

## Data Cleaning

This program strives to be as portable as possible so it does the following data cleaning:
- Month names are converted from "January" to 1, "February" to 2, etc.
- Empty values are set to NULL in database
- Fake emails - lots of rows had "True" or "False" as email, converts to NULL
- Missing years - some records missing active_year or year_registered

## Using the Query Interface

After import, you can run the SQL queries that are listed below:

```sql
-- Count total patrons
SELECT COUNT(*) FROM patrons;

-- See patron types
SELECT patron_type_def, COUNT(*) as count 
FROM patrons 
GROUP BY patron_type_def;

-- Most popular libraries
SELECT home_library_def, COUNT(*) as count 
FROM patrons 
GROUP BY home_library_def 
ORDER BY count DESC 
LIMIT 10;

-- Age distribution
SELECT age_range, COUNT(*) as count 
FROM patrons 
GROUP BY age_range 
ORDER BY count DESC;
```

Type `help` for more example queries, or `exit` to quit.

## Key Functions

- `runScripts()` - Runs SQL files to create tables
- `importExcel()` - Reads Excel and imports data
- `monthToIntOrNull()` - Converts month names to numbers
- `cleanEmail()` - Filters out invalid emails
- `startTextInterface()` - The query interface

## Common Issues

**"Connection refused"?**
MySQL isn't running. Start it with `mysql.server start` or check your MySQL installation.

**"No database selected"**
Running benchmarks has a chance to disconnect the database. Simply relaunch the program to fix this. 

## Notes

- Password is hardcoded (not ideal but fine for an assignment like this)
- Query interface lets you run any SQL which is not advisable outside of this setting
- Import takes does take some time as the database is large. Around 30 seconds on my M1 Macbook.
- Data is wiped an reimported each time the application is run to increase portability.

## Future ideas

- Import is rather slow right now taking around 30 seconds. Planning to implement goroutines in the future with the Mongo assignment. 