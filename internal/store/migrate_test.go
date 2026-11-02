package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tosrv/pmess/internal/store/storetest"
)

func TestMigrateCreatesSchema(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	var used int
	if err := s.Pool().QueryRow(ctx, `SELECT used FROM user_slots WHERE id=1`).Scan(&used); err != nil || used != 0 {
		t.Fatalf("used=%d err=%v", used, err)
	}
	if _, err := s.Pool().Exec(ctx, `UPDATE user_slots SET used=301`); err == nil {
		t.Fatal("expected check violation")
	}
	var uid, cid string
	if err := s.Pool().QueryRow(ctx, `INSERT INTO users(kind,email,display_name) VALUES('guest','a@b.c','G1') RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool().QueryRow(ctx, `INSERT INTO users(kind,display_name) VALUES('guest','G2') RETURNING id`).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	a, b := uid, cid
	if a > b {
		a, b = b, a
	}
	var conv string
	if err := s.Pool().QueryRow(ctx, `INSERT INTO conversations(user_a,user_b) VALUES($1,$2) RETURNING id`, a, b).Scan(&conv); err != nil {
		t.Fatal(err)
	}
	ins := `INSERT INTO messages(conversation_id,seq,sender_id,client_msg_id,body) VALUES($1,1,$2,gen_random_uuid(),$3)`
	if _, err := s.Pool().Exec(ctx, ins, conv, uid, strings.Repeat("x", 2001)); err == nil {
		t.Fatal("expected error for 2001-char body")
	}
	if _, err := s.Pool().Exec(ctx, ins, conv, uid, strings.Repeat("x", 2000)); err != nil {
		t.Fatalf("2000 chars should pass: %v", err)
	}
}
