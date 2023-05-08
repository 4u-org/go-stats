package bot

import (
	"context"
	"encoding/binary"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/gotd/contrib/bbolt"
	"github.com/gotd/td/telegram/updates"
)

func i2b(v int) []byte { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, uint64(v)); return b }

func b2i(b []byte) int { return int(binary.LittleEndian.Uint64(b)) }

func i642b(v int64) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return b
}

func b2i64(b []byte) int64 { return int64(binary.LittleEndian.Uint64(b)) }

var _ updates.StateStorage = (*BoltState)(nil)

type BoltState struct {
	db *bolt.DB
}

func NewBoltState(db *bolt.DB) *BoltState { return &BoltState{db} }

func (s *BoltState) GetState(ctx context.Context, userID int64) (state updates.State, found bool, err error) {
	tx, err := s.db.Begin(false)
	if err != nil {
		return updates.State{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	user := tx.Bucket(i642b(userID))
	if user == nil {
		return updates.State{}, false, nil
	}

	stateBucket := user.Bucket([]byte("state"))
	if stateBucket == nil {
		return updates.State{}, false, nil
	}

	var (
		pts  = stateBucket.Get([]byte("pts"))
		qts  = stateBucket.Get([]byte("qts"))
		date = stateBucket.Get([]byte("date"))
		seq  = stateBucket.Get([]byte("seq"))
	)

	if pts == nil || qts == nil || date == nil || seq == nil {
		return updates.State{}, false, nil
	}

	return updates.State{
		Pts:  b2i(pts),
		Qts:  b2i(qts),
		Date: b2i(date),
		Seq:  b2i(seq),
	}, true, nil
}

func (s *BoltState) SetState(ctx context.Context, userID int64, state updates.State) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		b, err := user.CreateBucketIfNotExists([]byte("state"))
		if err != nil {
			return err
		}

		check := func(e error) {
			if err != nil {
				return
			}
			err = e
		}

		check(b.Put([]byte("pts"), i2b(state.Pts)))
		check(b.Put([]byte("qts"), i2b(state.Qts)))
		check(b.Put([]byte("date"), i2b(state.Date)))
		check(b.Put([]byte("seq"), i2b(state.Seq)))
		return err
	})
}

func (s *BoltState) SetPts(ctx context.Context, userID int64, pts int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		state := user.Bucket([]byte("state"))
		if state == nil {
			return fmt.Errorf("internalState not found")
		}
		return state.Put([]byte("pts"), i2b(pts))
	})
}

func (s *BoltState) SetQts(ctx context.Context, userID int64, qts int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		state := user.Bucket([]byte("state"))
		if state == nil {
			return fmt.Errorf("internalState not found")
		}
		return state.Put([]byte("qts"), i2b(qts))
	})
}

func (s *BoltState) SetDate(ctx context.Context, userID int64, date int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		state := user.Bucket([]byte("state"))
		if state == nil {
			return fmt.Errorf("internalState not found")
		}
		return state.Put([]byte("date"), i2b(date))
	})
}

func (s *BoltState) SetSeq(ctx context.Context, userID int64, seq int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		state := user.Bucket([]byte("state"))
		if state == nil {
			return fmt.Errorf("internalState not found")
		}
		return state.Put([]byte("seq"), i2b(seq))
	})
}

func (s *BoltState) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		state := user.Bucket([]byte("state"))
		if state == nil {
			return fmt.Errorf("internalState not found")
		}
		if err := state.Put([]byte("date"), i2b(date)); err != nil {
			return err
		}
		return state.Put([]byte("seq"), i2b(seq))
	})
}

func (s *BoltState) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	tx, err := s.db.Begin(false)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	user := tx.Bucket(i642b(userID))
	if user == nil {
		return 0, false, nil
	}

	channels := user.Bucket([]byte("channels"))
	if channels == nil {
		return 0, false, nil
	}

	pts := channels.Get(i642b(channelID))
	if pts == nil {
		return 0, false, nil
	}
	return b2i(pts), true, nil
}

func (s *BoltState) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		channels, err := user.CreateBucketIfNotExists([]byte("channels"))
		if err != nil {
			return err
		}
		return channels.Put(i642b(channelID), i2b(pts))
	})
}

func (s *BoltState) DeleteChannelPts(ctx context.Context, userID, channelID int64) error {
	fmt.Println("DeleteChannelPts", userID, channelID)
	return s.db.Update(func(tx *bolt.Tx) error {
		user := tx.Bucket(i642b(userID))
		if user == nil {
			return nil
		}

		channels := user.Bucket([]byte("channels"))
		if channels == nil {
			return nil
		}

		return channels.Delete(i642b(channelID))
	})
}

func (s *BoltState) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		channels, err := user.CreateBucketIfNotExists([]byte("channels"))
		if err != nil {
			return err
		}

		return channels.ForEach(func(k, v []byte) error {
			fmt.Println("ForEachChannels", userID, b2i64(k), b2i(v))
			return f(ctx, b2i64(k), b2i(v))
		})
	})
}

var _ updates.ChannelAccessHasher = (*BoltAccessHasher)(nil)

type BoltAccessHasher struct {
	db *bolt.DB
}

func NewBoltAccessHasher(db *bolt.DB) *BoltAccessHasher { return &BoltAccessHasher{db} }

func (s *BoltAccessHasher) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (accessHash int64, found bool, err error) {
	tx, err := s.db.Begin(false)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	user := tx.Bucket(i642b(userID))
	if user == nil {
		return 0, false, nil
	}

	channels := user.Bucket([]byte("channelsHashes"))
	if channels == nil {
		return 0, false, nil
	}

	pts := channels.Get(i642b(channelID))
	if pts == nil {
		return 0, false, nil
	}

	return b2i64(pts), true, nil
}

func (s *BoltAccessHasher) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		user, err := tx.CreateBucketIfNotExists(i642b(userID))
		if err != nil {
			return err
		}

		channels, err := user.CreateBucketIfNotExists([]byte("channelsHashes"))
		if err != nil {
			return err
		}
		return channels.Put(i642b(channelID), i642b(accessHash))
	})
}

func NewBoltStorage(db *bolt.DB, userID int64) *bbolt.SessionStorage {
	storage := bbolt.NewSessionStorage(db, "session", i642b(userID))
	return &storage
}
