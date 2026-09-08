package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	Dir string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "state.db")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, err
	}
	// One SQLite writer; SSH/model/transfer workers do not share this execution limit.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Dir: dir}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS documents(kind TEXT NOT NULL,id TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(kind,id));
CREATE TABLE IF NOT EXISTS events(seq INTEGER PRIMARY KEY AUTOINCREMENT,task_id TEXT NOT NULL,kind TEXT NOT NULL,text TEXT NOT NULL,created TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS events_task ON events(task_id,seq);
CREATE TABLE IF NOT EXISTS executions(id TEXT PRIMARY KEY,task_id TEXT NOT NULL,call_id TEXT NOT NULL,digest TEXT NOT NULL,payload BLOB NOT NULL,UNIQUE(task_id,call_id));
CREATE TABLE IF NOT EXISTS checkpoints(id TEXT PRIMARY KEY,payload BLOB NOT NULL);
PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Put(kind, id string, v any) error {
	if id == "" {
		return errors.New("empty record id")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO documents(kind,id,payload) VALUES(?,?,?) ON CONFLICT(kind,id) DO UPDATE SET payload=excluded.payload`, kind, id, b)
	return err
}
func (s *Store) Load(kind, id string, v any) error {
	var b []byte
	err := s.db.QueryRow(`SELECT payload FROM documents WHERE kind=? AND id=?`, kind, id).Scan(&b)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func All[T any](s *Store, kind string) ([]T, error) {
	rows, err := s.db.Query(`SELECT payload FROM documents WHERE kind=? ORDER BY id`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var b []byte
		var v T
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Delete(kind, id string) error {
	_, err := s.db.Exec(`DELETE FROM documents WHERE kind=? AND id=?`, kind, id)
	return err
}
func (s *Store) Host(id string) (domain.Host, error) {
	var h domain.Host
	err := s.Load("hosts", id, &h)
	return h, err
}
func (s *Store) Event(task, kind, text string) (domain.Event, error) {
	e := domain.Event{TaskID: task, Kind: kind, Text: text, Time: time.Now().UTC()}
	r, err := s.db.Exec(`INSERT INTO events(task_id,kind,text,created) VALUES(?,?,?,?)`, task, kind, text, e.Time.Format(time.RFC3339Nano))
	if err != nil {
		return e, err
	}
	e.Sequence, err = r.LastInsertId()
	return e, err
}
func (s *Store) Events(task string, after int64) ([]domain.Event, error) {
	rows, err := s.db.Query(`SELECT seq,task_id,kind,text,created FROM events WHERE task_id=? AND seq>? ORDER BY seq`, task, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Event{}
	for rows.Next() {
		var e domain.Event
		var ts string
		if err = rows.Scan(&e.Sequence, &e.TaskID, &e.Kind, &e.Text, &ts); err != nil {
			return nil, err
		}
		e.Time, _ = time.Parse(time.RFC3339Nano, ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Claim records intent before contacting a server. A duplicate call is never redispatched.
func (s *Store) Claim(req domain.Request) (domain.Result, bool, error) {
	r := domain.Result{ID: domain.ID(), Request: req, Status: "running", ExitCode: -1, StartedAt: time.Now().UTC()}
	b, _ := json.Marshal(r)
	_, err := s.db.Exec(`INSERT INTO executions(id,task_id,call_id,digest,payload) VALUES(?,?,?,?,?) ON CONFLICT(task_id,call_id) DO NOTHING`, r.ID, req.TaskID, req.CallID, domain.Digest(req), b)
	if err != nil {
		return r, false, err
	}
	var previous []byte
	var digest string
	err = s.db.QueryRow(`SELECT digest,payload FROM executions WHERE task_id=? AND call_id=?`, req.TaskID, req.CallID).Scan(&digest, &previous)
	if err != nil {
		return r, false, err
	}
	if digest != domain.Digest(req) {
		return r, false, errors.New("tool call identity reused with different arguments")
	}
	var actual domain.Result
	if err = json.Unmarshal(previous, &actual); err != nil {
		return r, false, err
	}
	return actual, actual.ID == r.ID, nil
}
func (s *Store) Finish(r domain.Result) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE executions SET payload=? WHERE id=?`, b, r.ID)
	return err
}
func (s *Store) Results(task string) ([]domain.Result, error) {
	rows, err := s.db.Query(`SELECT payload FROM executions WHERE task_id=? ORDER BY rowid`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Result{}
	for rows.Next() {
		var b []byte
		var r domain.Result
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) Recover() error {
	rows, err := s.db.Query(`SELECT payload FROM executions`)
	if err != nil {
		return err
	}
	var pending []domain.Result
	for rows.Next() {
		var b []byte
		var r domain.Result
		if err = rows.Scan(&b); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(b, &r); err != nil {
			rows.Close()
			return err
		}
		if r.Status == "running" {
			pending = append(pending, r)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range pending {
		r.Status = "unknown"
		r.Error = "应用已退出，远端结果需要核实"
		if err = s.Finish(r); err != nil {
			return err
		}
	}
	tasks, err := All[domain.Task](s, "tasks")
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.Status == "running" || t.Status == "awaiting_approval" {
			t.Status = "interrupted"
		}
		t.Grant.ExpiresAt = time.Time{}
		if err = s.Put("tasks", t.ID, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) ([]byte, bool, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM checkpoints WHERE id=?`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return b, err == nil, err
}
func (s *Store) Set(ctx context.Context, id string, b []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO checkpoints(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, id, b)
	return err
}

type Checkpoints struct{ *Store }

func (s Checkpoints) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM checkpoints WHERE id=?`, id)
	return err
}
func (s *Store) Artifact(id string) (string, *os.File, error) {
	if id == "" || filepath.Base(id) != id {
		return "", nil, fmt.Errorf("invalid artifact id")
	}
	d := filepath.Join(s.Dir, "artifacts")
	if err := os.MkdirAll(d, 0700); err != nil {
		return "", nil, err
	}
	p := filepath.Join(d, id+".log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	return p, f, err
}
