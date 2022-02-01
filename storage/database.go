package storage

import (
	"errors"
	"fmt"

	"github.com/boltdb/bolt"
)

const (
	// DefaultDbPath is the path to the timbre database file.
	DefaultDbPath = "../timbre.db"
	// DefaultTestDbPath is the path to the test timbre database file.
	DefaultTestDbPath = "../timbre-test.db"
)

// Database wraps a boltdb connection and provides methods for accessing the database.
type Database struct {
	connection *bolt.DB
}

// OpenDatabase opens a boltdb connection based on the file path. The file will be created if it does not already exist.
func OpenDatabase(dbPath string) (*Database, error) {
	connection, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &Database{connection: connection}, nil
}

// Close closes the database connection
func (db *Database) Close() {
	db.connection.Close()
}

// HasBucket checks if the designated bucket exists.
func (db *Database) HasBucket(name []byte) bool {
	err := db.connection.View(func(tx *bolt.Tx) error {
		if tx.Bucket(name) != nil {
			return nil
		}
		return bolt.ErrBucketNotFound
	})
	return err == nil
}

// NewBucket creates a new bucket in the database. It will return an error if the bucket already exists.
func (db *Database) NewBucket(name []byte) error {
	err := db.connection.Update(func(tx *bolt.Tx) error {
		if tx.Bucket(name) != nil {
			return fmt.Errorf("error creating bucket %s - bucket already exists", name)
		}
		_, err := tx.CreateBucket(name)
		return err
	})
	return err
}

// DeleteBucket deletes the designated bucket.
func (db *Database) DeleteBucket(name []byte) error {
	err := db.connection.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket(name)
	})
	return err
}

// Put puts a new key-value pair in the designated bucket. Put is not allowed to overwrite existing key-value pairs.
func (db *Database) Put(bucketName, key, value []byte) error {
	err := db.connection.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("put error - bucket %s does not exist", bucketName)
		}
		if b.Get(key) != nil {
			return fmt.Errorf("put error - key %x already exists", key)
		}
		return b.Put(key, value)
	})
	return err
}

// BatchPut puts multiple key-value pairs in the designated bucket. Batchput is not allowed to overwrite existing key-value pairs.
func (db *Database) BatchPut(bucketName []byte, keys [][]byte, values [][]byte) error {
	if len(keys) != len(values) {
		return errors.New("batchput error - keys and values have different lengths")
	}
	err := db.connection.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("batchput error - bucket %s does not exist", bucketName)
		}
		for i, key := range keys {
			value := values[i]
			if b.Get(key) != nil {
				return fmt.Errorf("batchput error - key %x already exists", key)
			}
			if err := b.Put(key, value); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// Get returns the value corresponding to the given key in the designated bucket. In contrast to boltdb, Get will return an error if key does not exist.
func (db *Database) Get(bucketName, key []byte) ([]byte, error) {
	var value []byte
	err := db.connection.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("get error - bucket %s does not exist", bucketName)
		}
		value = b.Get(key)
		if value == nil {
			return fmt.Errorf("get error - key %x does not exist", key)
		}
		return nil
	})
	return value, err
}

// ForEach executes the given READ-ONLY function on every key-value pair in the designated bucket.
func (db *Database) ForEach(bucketName []byte, fn func(k []byte, v []byte) error) error {
	err := db.connection.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("foreach error - bucket %s does not exist", bucketName)
		}
		return b.ForEach(fn)
	})
	return err
}

// Get returns the value corresponding to the given key in the designated bucket. In contrast to boltdb, Get will return an error if key does not exist.
func (db *Database) Delete(bucketName, key []byte) error {
	err := db.connection.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("Delete error - bucket %s does not exist", bucketName)
		}
		return b.Delete(key)
	})
	return err
}
