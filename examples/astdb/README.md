# AST to sqlite3 Database

## Usage

```bash
go run main.go scan -D /path/to/src -o /path/to/output.db
```

## Development

### Database

Create a new model (table)

```bash
go run -mod=mod entgo.io/ent/cmd/ent new YourModelName
```

Generate the database schema and access code

```bash
go generate ./ent
```

For database access, use `sqlite3` storage adapter from `./storage`
