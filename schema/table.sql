-- Active: 1727369093383@@127.0.0.1@5433@postgres
CREATE TABLE IF NOT EXISTS CVs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(15) NOT NULL,
    age INT NOT NULL,
    surname VARCHAR(20) NOT NULL,
    email_cv VARCHAR(40) NOT NULL,
    living_city VARCHAR(30) NOT NULL,
    profession VARCHAR(20) NOT NULL,
    salary INT NOT NULL,
    phone_number VARCHAR(15),
    education VARCHAR(50),
    skills TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(10),
    hash_password VARCHAR(70) NOT NULL,
    email VARCHAR(20) NOT NULL 
);

DROP TABLE IF EXISTS CVs;

DROP TABLE IF EXISTS users;