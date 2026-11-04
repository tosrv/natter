package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func mkUsers(t *testing.T, s *store.Store, n int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		u, _, err := s.UpsertGoogleUser(context.Background(), fmt.Sprintf("sub-%d", i), fmt.Sprintf("u%d@x.io", i), "", false)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = u.ID
	}
	return ids
}

func checkConv(t *testing.T, s *store.Store, conv uuid.UUID, want int) {
	t.Helper()
	ctx := context.Background()
	var cnt, minSeq, maxSeq, distinctSeq, distinctClient int64
	err := s.Pool().QueryRow(ctx, `SELECT count(*), coalesce(min(seq),0), coalesce(max(seq),0),
		count(DISTINCT seq), count(DISTINCT (sender_id, client_msg_id)) FROM messages WHERE conversation_id=$1`, conv).
		Scan(&cnt, &minSeq, &maxSeq, &distinctSeq, &distinctClient)
	if err != nil {
		t.Fatal(err)
	}
	if cnt != int64(want) || minSeq != 1 || maxSeq != int64(want) || distinctSeq != cnt || distinctClient != cnt {
		t.Fatalf("conv %s: count=%d min=%d max=%d distinctSeq=%d distinctClient=%d want %d",
			conv, cnt, minSeq, maxSeq, distinctSeq, distinctClient, want)
	}
	var lastSeq int64
	s.Pool().QueryRow(ctx, `SELECT last_seq FROM conversations WHERE id=$1`, conv).Scan(&lastSeq)
	if lastSeq != int64(want) {
		t.Fatalf("last_seq=%d want %d", lastSeq, want)
	}
}

func TestConcurrentSend(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	users := mkUsers(t, s, 50)
	pairs := make([]uuid.UUID, 25)
	for i := range pairs {
		c, err := s.GetOrCreateConversation(ctx, users[2*i], users[2*i+1])
		if err != nil {
			t.Fatal(err)
		}
		pairs[i] = c
	}
	shared, err := s.GetOrCreateConversation(ctx, users[0], users[49])
	if err != nil {
		t.Fatal(err)
	}
	const per = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	errc := make(chan error, 50*per*2)
	for i, u := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for k := 0; k < per; k++ {
				for _, c := range []uuid.UUID{pairs[i/2], shared} {
					_, dup, err := s.SendMessage(ctx, c, u, uuid.New(), fmt.Sprintf("m%d", k))
					if err != nil || dup {
						errc <- fmt.Errorf("send: dup=%v err=%v", dup, err)
					}
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatal(err)
	}
	for _, c := range pairs {
		checkConv(t, s, c, 2*per)
	}
	checkConv(t, s, shared, 50*per)
}

func TestResend(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 2)
	c, err := s.GetOrCreateConversation(ctx, u[0], u[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, c, u[0], uuid.New(), "first"); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	m1, d1, err := s.SendMessage(ctx, c, u[0], cid, "hi")
	if err != nil || d1 {
		t.Fatalf("first send dup=%v err=%v", d1, err)
	}
	m2, d2, err := s.SendMessage(ctx, c, u[0], cid, "hi")
	if err != nil || !d2 || m2.ID != m1.ID || m2.Seq != 2 {
		t.Fatalf("resend dup=%v err=%v id eq=%v seq=%d", d2, err, m2.ID == m1.ID, m2.Seq)
	}
	msgs, err := s.ListMessages(ctx, c, 1, 0, 50)
	if err != nil || len(msgs) != 1 || msgs[0].ID != m1.ID || msgs[0].ClientMsgID != cid {
		t.Fatalf("list after: %v %+v", err, msgs)
	}

	// concurrent resend
	for round := 0; round < 20; round++ {
		cid := uuid.New()
		res := make([]store.Message, 2)
		dups := make([]bool, 2)
		errs := make([]error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				res[i], dups[i], errs[i] = s.SendMessage(ctx, c, u[0], cid, "race")
			}()
		}
		close(start)
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || res[0].ID != res[1].ID || dups[0] == dups[1] {
			t.Fatalf("round %d: errs=%v ids=%v,%v dups=%v", round, errs, res[0].ID, res[1].ID, dups)
		}
	}
	checkConv(t, s, c, 22)
}

func TestGetOrCreateConversationConcurrent(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 2)
	ids := make([]uuid.UUID, 10)
	errs := make([]error, 10)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			a, b := u[0], u[1]
			if i%2 == 1 {
				a, b = b, a
			}
			ids[i], errs[i] = s.GetOrCreateConversation(ctx, a, b)
		}()
	}
	close(start)
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("i=%d err=%v id=%v want %v", i, errs[i], ids[i], ids[0])
		}
	}
	var n int
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM conversation_members WHERE conversation_id=$1`, ids[0]).Scan(&n)
	if n != 2 {
		t.Fatalf("members=%d", n)
	}
	if ok, _ := s.IsMember(ctx, ids[0], u[0]); !ok {
		t.Fatal("not member")
	}
	if p, err := s.Peer(ctx, ids[0], u[0]); err != nil || p != u[1] {
		t.Fatalf("peer=%v err=%v", p, err)
	}
	other := mkUsersFrom(t, s, 100, 1)[0]
	if ok, _ := s.IsMember(ctx, ids[0], other); ok {
		t.Fatal("outsider member")
	}
	if _, err := s.Peer(ctx, ids[0], other); err != store.ErrNotFound {
		t.Fatalf("peer outsider err=%v", err)
	}
}

func mkUsersFrom(t *testing.T, s *store.Store, off, n int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		u, _, err := s.UpsertGoogleUser(context.Background(), fmt.Sprintf("sub-%d", off+i), "e@x.io", "", false)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = u.ID
	}
	return ids
}

func TestMarkReadMonotonic(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 2)
	c, _ := s.GetOrCreateConversation(ctx, u[0], u[1])
	for i := 0; i < 10; i++ {
		if _, _, err := s.SendMessage(ctx, c, u[0], uuid.New(), "x"); err != nil {
			t.Fatal(err)
		}
	}
	if v, err := s.MarkRead(ctx, c, u[1], 5); err != nil || v != 5 {
		t.Fatalf("v=%d err=%v", v, err)
	}
	if v, _ := s.MarkRead(ctx, c, u[1], 3); v != 5 {
		t.Fatalf("v=%d want 5", v)
	}
	if v, _ := s.MarkRead(ctx, c, u[1], 999); v != 10 {
		t.Fatalf("v=%d want 10", v)
	}
	if _, err := s.MarkRead(ctx, c, mkUsersFrom(t, s, 100, 1)[0], 1); err != store.ErrNotFound {
		t.Fatalf("outsider err=%v", err)
	}
}

func TestListMessagesAndConversations(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 3)
	c1, _ := s.GetOrCreateConversation(ctx, u[0], u[1])
	c2, _ := s.GetOrCreateConversation(ctx, u[0], u[2])
	for i := 0; i < 10; i++ {
		s.SendMessage(ctx, c1, u[1], uuid.New(), fmt.Sprint(i+1))
	}
	before, err := s.ListMessages(ctx, c1, 0, 8, 3)
	if err != nil || len(before) != 3 || before[0].Seq != 5 || before[2].Seq != 7 {
		t.Fatalf("before: %v %+v", err, before)
	}
	latest, _ := s.ListMessages(ctx, c1, 0, 0, 1000)
	if len(latest) != 10 || latest[0].Seq != 1 {
		t.Fatalf("latest len=%d", len(latest))
	}
	s.MarkRead(ctx, c1, u[0], 4)
	s.SendMessage(ctx, c2, u[2], uuid.New(), "hey")
	convs, err := s.ListConversations(ctx, u[0])
	if err != nil || len(convs) != 2 {
		t.Fatalf("convs: %v %d", err, len(convs))
	}
	if convs[0].ID != c2 || convs[1].ID != c1 {
		t.Fatalf("order wrong")
	}
	if convs[1].Unread != 6 || convs[1].PeerID != u[1] || convs[1].LastMessage == nil || convs[1].LastMessage.Body != "10" || convs[1].PeerKind != "google" {
		t.Fatalf("c1 summary %+v", convs[1])
	}
	empty, _ := s.ListConversations(ctx, u[1])
	if len(empty) != 1 || empty[0].PeerLastReadSeq != 4 {
		t.Fatalf("u1 view %+v", empty)
	}
}
