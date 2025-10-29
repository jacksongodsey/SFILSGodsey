# Findings

## Dataset Stats

Some statistics on the data that is imported:
- 437,115 total patrons
- 62% Adults, 13% Kids, 11% Seniors, 9% Teens
- 84.5% within San Francisco
- Main Library has 33% of all patrons (143k)
- Most patrons are age 25-44

# Performance Metrics

Did we store the data in our database appropriately?

Data is stored in multiple tables. 

## Performance Results
- Count all patrons: 76.034792ms (1 rows)
- Count by patron type: 146.570542ms (18 rows)
- Count by age range: 153.517209ms (11 rows)
- Count by library: 144.621667ms (30 rows)
- Find SF patrons: 54.954625ms (1 rows)
- Active in 2023: 52.88925ms (1 rows)