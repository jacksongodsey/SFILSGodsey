-- dropping tables for reliability
DROP TABLE IF EXISTS patrons;
DROP TABLE IF EXISTS patron_types;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS notification_types;

-- create supporting tables first

CREATE TABLE IF NOT EXISTS patron_types (
    id INT AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    description VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS libraries (
    code VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_types (
    code VARCHAR(50) PRIMARY KEY,
    description VARCHAR(255) NOT NULL
);

-- create main patrons table

CREATE TABLE IF NOT EXISTS patrons (
    id INT AUTO_INCREMENT PRIMARY KEY,
    patron_type_id INT NOT NULL,
    checkout_total INT DEFAULT 0,
    renewal_total INT DEFAULT 0,
    age_range VARCHAR(50),
    home_library_code VARCHAR(50),
    active_month INT NULL,
    active_year INT NULL,
    notification_type_code VARCHAR(50),
    email VARCHAR(255) NULL,
    within_sfc BOOLEAN DEFAULT 0,
    year_registered INT NULL,
    FOREIGN KEY (patron_type_id) REFERENCES patron_types(id),
    FOREIGN KEY (home_library_code) REFERENCES libraries(code),
    FOREIGN KEY (notification_type_code) REFERENCES notification_types(code)
);
