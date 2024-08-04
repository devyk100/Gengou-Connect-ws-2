package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"os"
)

type Db struct {
	conn *pgx.Conn
	ctx  context.Context
}

type LearnerRow struct {
	Id        pgtype.Int8
	FirstName pgtype.Text
	LastName  pgtype.Text
	Dob       pgtype.Date
	UserId    pgtype.Text
	Email     pgtype.Text
}

func (db *Db) InitDbConnection(ctx context.Context) {
	dbUrl := os.Getenv("DATABASE_URL")
	fmt.Printf("Connecting to %s\n", dbUrl)
	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		panic(err.Error())
	}
	//connChan <- conn
	db.conn = conn
	db.ctx = ctx
}

func (db *Db) Close() {
	err := db.conn.Close(db.ctx)
	if err != nil {
		panic(err.Error())
	}
}

func (db *Db) ExecSQL(sql string) {
	_, err := db.conn.Exec(context.Background(), sql)
	if err != nil {
		panic(err.Error())
	}
}

func (db *Db) QueryRow(sql string) pgx.Row {
	row := db.conn.QueryRow(db.ctx, sql)
	fmt.Println(row)
	return row
}

func (db *Db) Query(sql string) pgx.Rows {
	rows, err := db.conn.Query(db.ctx, sql)
	if err != nil {
		panic(err.Error())
	}
	return rows
}

func (db *Db) Exec(sql string) {
	_, err := db.conn.Exec(db.ctx, sql)
	if err != nil {
		panic(err.Error())
	}
}

func (db *Db) FetchLearner(userId string) LearnerRow {
	learner := LearnerRow{}
	row := db.conn.QueryRow(db.ctx, `SELECT * from learners WHERE userid = $1`, userId)
	err := row.Scan(&learner.Id, &learner.FirstName, &learner.LastName, &learner.Dob, &learner.UserId, &learner.Email)
	if err != nil {
		panic(err.Error())
		return LearnerRow{}
	}
	return learner
}
