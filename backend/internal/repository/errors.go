package repository

import "github.com/jackc/pgx/v5"

func ErrNoRows() error {
	return pgx.ErrNoRows
}
