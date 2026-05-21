package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Balance   int64
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WalletTransaction struct {
	ID           uuid.UUID
	WalletID     uuid.UUID
	Type         string
	Amount       int64
	BalanceAfter int64
	ReferenceID  uuid.UUID
	Description  string
	CreatedAt    time.Time
}

type WalletRepository interface {
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*Wallet, error)
	CreateWallet(ctx context.Context, userID uuid.UUID) (*Wallet, error)
	CreditWallet(ctx context.Context, userID uuid.UUID, amount int64, txType, description string, referenceID uuid.UUID) (*WalletTransaction, error)
	DebitWallet(ctx context.Context, userID uuid.UUID, amount int64, txType, description string, referenceID uuid.UUID) (*WalletTransaction, error)
}

type walletRepositoryImpl struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) WalletRepository {
	return &walletRepositoryImpl{db: db}
}

func (r *walletRepositoryImpl) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*Wallet, error) {
	query := `SELECT id, user_id, balance, is_active, created_at, updated_at FROM wallets WHERE user_id = $1`
	row := r.db.QueryRowContext(ctx, query, userID)

	var w Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Balance, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *walletRepositoryImpl) CreateWallet(ctx context.Context, userID uuid.UUID) (*Wallet, error) {
	query := `
		INSERT INTO wallets (user_id, balance, is_active)
		VALUES ($1, 0, true)
		RETURNING id, user_id, balance, is_active, created_at, updated_at
	`
	var w Wallet
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&w.ID, &w.UserID, &w.Balance, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *walletRepositoryImpl) CreditWallet(ctx context.Context, userID uuid.UUID, amount int64, txType, description string, referenceID uuid.UUID) (*WalletTransaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Lock wallet for update
	var w Wallet
	err = tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&w.ID, &w.Balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}

	// 2. Update balance
	newBalance := w.Balance + amount
	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = NOW() WHERE id = $2`, newBalance, w.ID)
	if err != nil {
		return nil, err
	}

	// 3. Create transaction record
	queryTx := `
		INSERT INTO wallet_transactions (wallet_id, type, amount, balance_after, reference_id, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, wallet_id, type, amount, balance_after, reference_id, description, created_at
	`
	var wTx WalletTransaction
	err = tx.QueryRowContext(ctx, queryTx, w.ID, txType, amount, newBalance, referenceID, description).
		Scan(&wTx.ID, &wTx.WalletID, &wTx.Type, &wTx.Amount, &wTx.BalanceAfter, &wTx.ReferenceID, &wTx.Description, &wTx.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &wTx, nil
}

func (r *walletRepositoryImpl) DebitWallet(ctx context.Context, userID uuid.UUID, amount int64, txType, description string, referenceID uuid.UUID) (*WalletTransaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Lock wallet for update
	var w Wallet
	err = tx.QueryRowContext(ctx, `SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&w.ID, &w.Balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}

	// 2. Check balance
	if w.Balance < amount {
		return nil, ErrInsufficientBalance
	}

	// 3. Update balance
	newBalance := w.Balance - amount
	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = $1, updated_at = NOW() WHERE id = $2`, newBalance, w.ID)
	if err != nil {
		return nil, err
	}

	// 4. Create transaction record
	queryTx := `
		INSERT INTO wallet_transactions (wallet_id, type, amount, balance_after, reference_id, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, wallet_id, type, amount, balance_after, reference_id, description, created_at
	`
	var wTx WalletTransaction
	// amount in transaction should be negative for debit conceptually, but we store it as absolute value usually or follow spec. Spec says "amount: IDR (positive = credit, negative = debit)". 
	// Let's store negative for debit.
	err = tx.QueryRowContext(ctx, queryTx, w.ID, txType, -amount, newBalance, referenceID, description).
		Scan(&wTx.ID, &wTx.WalletID, &wTx.Type, &wTx.Amount, &wTx.BalanceAfter, &wTx.ReferenceID, &wTx.Description, &wTx.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &wTx, nil
}
