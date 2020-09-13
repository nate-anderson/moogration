![Moo the database cow](https://github.com/nate-anderson/moogration/blob/master/moo.jpg | width=150)
# moogration
Simple MySQL migrations in Go. No dependencies, simple API, migration change detection and
status tracking, and a cute cow.

It might work with other SQL databases, but I haven't tried. I wrote this as part of another
project and put it in this repo for my own convenience of re-use. I doubt it's ready for
production.

## Defining migrations
Migrations should be defined using the `Migration` struct type and registered using the 
`Register` function, like so:

```go
moogration.Register(
    &Migration{
		Name: "001_create_table_user",
		Up: `CREATE TABLE user (
			...
		);`,
		Down: `DROP TABLE user;`,
    }
)
```

## Running migrations
Migrations registered with `Register` will be sorted ascending by the `name` key
and run in order. Previously run migrations will be skipped unless `force` is set to true.

```go
moogration.RunLatest(db, down, force)
```

This package assumes that database migrations are application critical and thus panics upon
encountering an error.
