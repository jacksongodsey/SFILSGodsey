# SFILS - San Francisco Library Patron Database

This is a go program that takes the San Francisco library patron data in an excel format and converts it into a MongoDB database which you can query.

## What It Does

- Reads data from an Excel file
- Creates and imports a MongoDB database
- Provides a very basic command line interface to run queries with MongoDB.
- Aims to handle data that isn't extremely coherent. 

## Setup

### Requirements
- Go installation
- MongoDB install and running locally
- The patron Excel file (`sfpl.xlsx`)

### Installation
- This program assumes that you are using MongoDB on your local machine on the default port which is 27017. You can export your connection string in your shell as seen below or hardcode it and other settings in the code.
```bash
# Install required Go packages.
go get go.mongodb.org/mongo-driver/mongo
go get go.mongodb.org/mongo-driver/bson
go get github.com/xuri/excelize/v2

# Set your MongoDB URI (optional - defaults to localhost:27017)
export MONGO_URI="mongodb://localhost:27017"

# Run the program
cd app
go run main.go
```

## Structure of the Project
```
project/
├── app/
│   └── main.go           # Main program
└── data/
    └── sfpl.xlsx         # Patron Excel file
```

## How It Works

1. Connects to MongoDB using credentials provided in the code or via a shell variable
2. Creates database called `sfils` if it does not currently exist
3. Creates indexes on collections for query performance
4. Imports Excel data and cleans it up and inserts the data into the database
5. Opens query interface where you can run MongoDB queries on the data

## Database Schema

The `patrons` collection has several fields which can be seen below:
- Standard info: patron type, age range
- Library: home library location
- Activity: checkouts, renewals, active month/year
- Other: email, within SF county, registration year

## Data Cleaning

This program strives to be as portable as possible so it does the following data cleaning:
- Month names are converted from "January" to 1, "February" to 2, etc.
- Empty values are set to null in database
- Fake emails - lots of rows had "True" or "False" as email, converts to null
- Missing years - some records missing active_year or year_registered

## Using the Query Interface

After import, you can run the MongoDB queries that are listed below:
```
-- Count total patrons
patrons|{}

-- Find patrons by type
patrons|{"patron_type_code": "ADULT"}

-- Find SF patrons
patrons|{"within_sfc": true}

-- Find patrons by age range
patrons|{"age_range": "25 to 34 years"}

-- Find gmail users
patrons|{"email": {"$regex": "gmail.com"}}

-- List all libraries
libraries|{}
```

Type `help` for more example queries, or `exit` to quit.

## Key Functions

- `createIndexes()` - Creates indexes on collections for performance
- `importExcel()` - Reads Excel and imports data
- `monthToIntOrNull()` - Converts month names to numbers
- `cleanEmail()` - Filters out invalid emails
- `startTextInterface()` - The query interface

## Common Issues

**"Connection refused"?**
MongoDB isn't running. Start it with `mongod` or check your MongoDB installation.

**Need to reset data?**
Simply relaunch the program - it drops and recreates collections on each run.

## Notes

- Connection string is hardcoded (not ideal but fine for an assignment like this)
- Query interface lets you run any MongoDB query which is not advisable outside of this setting
- Import takes does take some time as the database is large. Around 30 seconds on my M1 Macbook.
- Data is wiped and reimported each time the application is run to increase portability.