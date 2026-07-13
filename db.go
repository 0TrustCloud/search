package main

import (
	"os"
	"path/filepath"

	"github.com/0TrustCloud/ultimate_db"
)

func openDatabase(dbPath, walPath string) (*ultimate_db.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(walPath), 0o755); err != nil {
		return nil, err
	}

	device, err := ultimate_db.NewOSFileDevice(dbPath)
	if err != nil {
		return nil, err
	}
	dm := ultimate_db.NewDiskManager(device)
	evictor := ultimate_db.NewLRUEvictionPolicy()
	metrics := ultimate_db.NewAtomicMetrics()
	bp := ultimate_db.NewBufferPool(dm, 1024, evictor, metrics)
	wal, err := ultimate_db.NewBatchingWAL(walPath)
	if err != nil {
		return nil, err
	}
	db := ultimate_db.NewDB(bp, wal, metrics)
	if err := ultimate_db.PerformRecovery(db, walPath); err != nil {
		return nil, err
	}

	// Pre-allocate pages used by orchid_sync inverted index + metadata.
	for {
		p, err := bp.NewPage()
		if err != nil {
			return nil, err
		}
		bp.UnpinPage(p.ID, true)
		if p.ID >= ultimate_db.PageID(12) {
			break
		}
	}
	return db, nil
}
