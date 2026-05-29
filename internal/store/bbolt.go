package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/nickzhog/programmable-assistant/internal/fsm"
	"github.com/nickzhog/programmable-assistant/internal/session"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketSessions      = []byte("sessions")
	bucketNavPaths      = []byte("nav_paths")
	bucketFSMStates     = []byte("fsm_states")
	bucketActiveSession = []byte("active_sessions")
	bucketCallbackRefs  = []byte("callback_refs")
)

type BoltStore struct {
	db *bolt.DB
}

func Open(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{bucketSessions, bucketNavPaths, bucketFSMStates, bucketActiveSession, bucketCallbackRefs}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", b, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

func userKey(userID int64) []byte {
	return []byte(strconv.FormatInt(userID, 10))
}

func (s *BoltStore) SaveSession(ctx context.Context, sess *session.Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		return b.Put([]byte(sess.ID), data)
	})
}

func (s *BoltStore) GetSession(ctx context.Context, id string) (*session.Session, error) {
	var sess session.Session
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &sess)
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return &sess, nil
}

func (s *BoltStore) ListSessions(ctx context.Context, userID int64) ([]*session.Session, error) {
	var sessions []*session.Session
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var sess session.Session
			if err := json.Unmarshal(v, &sess); err != nil {
				continue
			}
			if sess.UserID == userID {
				sessions = append(sessions, &sess)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []*session.Session{}
	}
	return sessions, nil
}

func (s *BoltStore) DeleteSession(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) GetNavPath(ctx context.Context, userID int64) (string, error) {
	var path string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNavPaths)
		data := b.Get(userKey(userID))
		if data != nil {
			path = string(data)
		}
		return nil
	})
	return path, err
}

func (s *BoltStore) SetNavPath(ctx context.Context, userID int64, path string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNavPaths)
		return b.Put(userKey(userID), []byte(path))
	})
}

func (s *BoltStore) GetFSMState(ctx context.Context, userID int64) (*fsm.State, error) {
	var state fsm.State
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketFSMStates)
		data := b.Get(userKey(userID))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &state)
	})
	if err != nil || !found {
		return nil, err
	}
	return &state, nil
}

func (s *BoltStore) SetFSMState(ctx context.Context, userID int64, state *fsm.State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal fsm state: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketFSMStates)
		return b.Put(userKey(userID), data)
	})
}

func (s *BoltStore) ClearFSMState(ctx context.Context, userID int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketFSMStates)
		return b.Delete(userKey(userID))
	})
}

func (s *BoltStore) GetActiveSessionID(ctx context.Context, userID int64) (string, error) {
	var id string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketActiveSession)
		data := b.Get(userKey(userID))
		if data != nil {
			id = string(data)
		}
		return nil
	})
	return id, err
}

func (s *BoltStore) SetActiveSessionID(ctx context.Context, userID int64, sessionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketActiveSession)
		return b.Put(userKey(userID), []byte(sessionID))
	})
}

func (s *BoltStore) SetCallbackRef(ctx context.Context, key string, value string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCallbackRefs)
		return b.Put([]byte(key), []byte(value))
	})
}

func (s *BoltStore) GetCallbackRef(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCallbackRefs)
		data := b.Get([]byte(key))
		if data != nil {
			val = string(data)
		}
		return nil
	})
	return val, err
}
