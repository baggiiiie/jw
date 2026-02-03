package config

import "sync"

type ConfigStore interface {
	Load() (*Config, error)
	Save(*Config) error
	Update(func(*Config) error) error
}

type DiskStore struct {
	mu sync.Mutex
}

func NewDiskStore() *DiskStore {
	return &DiskStore{}
}

func (s *DiskStore) Load() (*Config, error) {
	var cfg *Config
	err := s.withLock(func() error {
		var loadErr error
		cfg, loadErr = loadFromDisk()
		return loadErr
	})
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *DiskStore) Save(cfg *Config) error {
	return s.withLock(func() error {
		return saveToDisk(cfg)
	})
}

func (s *DiskStore) Update(fn func(*Config) error) error {
	return s.withLock(func() error {
		cfg, err := loadFromDisk()
		if err != nil {
			return err
		}
		if err := fn(cfg); err != nil {
			return err
		}
		return saveToDisk(cfg)
	})
}

func (s *DiskStore) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return withFileLock(fn)
}
