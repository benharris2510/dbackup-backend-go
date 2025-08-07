-- Initialize MySQL database for testing backup functionality
USE testdb;

-- Create sample tables for testing backups
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    total_amount DECIMAL(10, 2),
    status ENUM('pending', 'completed', 'cancelled') DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO users (username, email, password_hash) VALUES
('john_doe', 'john@example.com', '$2a$10$example_hash_1'),
('jane_smith', 'jane@example.com', '$2a$10$example_hash_2'),
('bob_wilson', 'bob@example.com', '$2a$10$example_hash_3')
ON DUPLICATE KEY UPDATE username=username;

INSERT INTO products (name, description, price, stock) VALUES
('Laptop', 'High-performance laptop', 999.99, 50),
('Mouse', 'Wireless optical mouse', 29.99, 100),
('Keyboard', 'Mechanical gaming keyboard', 149.99, 75)
ON DUPLICATE KEY UPDATE name=name;

INSERT INTO orders (user_id, total_amount, status) VALUES
(1, 999.99, 'completed'),
(2, 179.98, 'pending'),
(1, 29.99, 'completed')
ON DUPLICATE KEY UPDATE total_amount=total_amount;