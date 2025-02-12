import sqlite3

conn = sqlite3.connect('example.db')
cursor = conn.cursor()
cursor.execute("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, data TEXT)")
cursor.execute("INSERT INTO test (data) VALUES (?)", ("Hello, World!",))
conn.commit()
conn.close()