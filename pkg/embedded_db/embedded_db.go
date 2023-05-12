package embedded_db

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

func CallDatabase() (*badger.DB, error) {
	db, err := badger.Open(badger.DefaultOptions("/tmp/badgerv4"))
	return db, err
}

func ViewDatabase(db *badger.DB) {
	_ = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				fmt.Printf("key=%s, value=%s\n", k, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

}

func SearchDatabase(db *badger.DB, key string) (string, error) {
	var valCopy []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			valCopy = append([]byte{}, val...)
			return nil
		})

		if err != nil {
			fmt.Println("ERROR at getting value from item")
		}
		return err
	})
	return string(valCopy), err
}

func UpdateDatabase(db *badger.DB, key string, value string) error {
	err := db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(string(key)), []byte(value))
		return err
	})
	return err
}

func DeleteDatabase(db *badger.DB, key string) error {
	err := db.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(key))
		return err
	})
	return err
}
