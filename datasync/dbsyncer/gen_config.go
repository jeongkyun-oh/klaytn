// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package dbsyncer

import (
	"time"
)

// MarshalTOML marshals as TOML.
func (d DBConfig) MarshalTOML() (interface{}, error) {
	type DBConfig struct {
		EnabledDBSyncer  bool
		EnabledLogMode   bool
		DBHost           string        `toml:",omitempty"`
		DBPort           string        `toml:",omitempty"`
		DBUser           string        `toml:",omitempty"`
		DBPassword       string        `toml:",omitempty"`
		DBName           string        `toml:",omitempty"`
		MaxIdleConns     int           `toml:",omitempty"`
		MaxOpenConns     int           `toml:",omitempty"`
		ConnMaxLifetime  time.Duration `toml:",omitempty"`
		BlockChannelSize int           `toml:",omitempty"`
		GenQueryThread   int           `toml:",omitempty"`
		InsertThread     int           `toml:",omitempty"`
		BulkInsertSize   int           `toml:",omitempty"`
		Mode             string        `toml:",omitempty"`
		EventMode        string        `toml:",omitempty"`
	}
	var enc DBConfig
	enc.EnabledDBSyncer = d.EnabledDBSyncer
	enc.EnabledLogMode = d.EnabledLogMode
	enc.DBHost = d.DBHost
	enc.DBPort = d.DBPort
	enc.DBUser = d.DBUser
	enc.DBPassword = d.DBPassword
	enc.DBName = d.DBName
	enc.MaxIdleConns = d.MaxIdleConns
	enc.MaxOpenConns = d.MaxOpenConns
	enc.ConnMaxLifetime = d.ConnMaxLifetime
	enc.BlockChannelSize = d.BlockChannelSize
	enc.GenQueryThread = d.GenQueryThread
	enc.InsertThread = d.InsertThread
	enc.BulkInsertSize = d.BulkInsertSize
	enc.Mode = d.Mode
	enc.EventMode = d.EventMode
	return &enc, nil
}

// UnmarshalTOML unmarshals from TOML.
func (d *DBConfig) UnmarshalTOML(unmarshal func(interface{}) error) error {
	type DBConfig struct {
		EnabledDBSyncer  *bool
		EnabledLogMode   *bool
		DBHost           *string        `toml:",omitempty"`
		DBPort           *string        `toml:",omitempty"`
		DBUser           *string        `toml:",omitempty"`
		DBPassword       *string        `toml:",omitempty"`
		DBName           *string        `toml:",omitempty"`
		MaxIdleConns     *int           `toml:",omitempty"`
		MaxOpenConns     *int           `toml:",omitempty"`
		ConnMaxLifetime  *time.Duration `toml:",omitempty"`
		BlockChannelSize *int           `toml:",omitempty"`
		GenQueryThread   *int           `toml:",omitempty"`
		InsertThread     *int           `toml:",omitempty"`
		BulkInsertSize   *int           `toml:",omitempty"`
		Mode             *string        `toml:",omitempty"`
		EventMode        *string        `toml:",omitempty"`
	}
	var dec DBConfig
	if err := unmarshal(&dec); err != nil {
		return err
	}
	if dec.EnabledDBSyncer != nil {
		d.EnabledDBSyncer = *dec.EnabledDBSyncer
	}
	if dec.EnabledLogMode != nil {
		d.EnabledLogMode = *dec.EnabledLogMode
	}
	if dec.DBHost != nil {
		d.DBHost = *dec.DBHost
	}
	if dec.DBPort != nil {
		d.DBPort = *dec.DBPort
	}
	if dec.DBUser != nil {
		d.DBUser = *dec.DBUser
	}
	if dec.DBPassword != nil {
		d.DBPassword = *dec.DBPassword
	}
	if dec.DBName != nil {
		d.DBName = *dec.DBName
	}
	if dec.MaxIdleConns != nil {
		d.MaxIdleConns = *dec.MaxIdleConns
	}
	if dec.MaxOpenConns != nil {
		d.MaxOpenConns = *dec.MaxOpenConns
	}
	if dec.ConnMaxLifetime != nil {
		d.ConnMaxLifetime = *dec.ConnMaxLifetime
	}
	if dec.BlockChannelSize != nil {
		d.BlockChannelSize = *dec.BlockChannelSize
	}
	if dec.GenQueryThread != nil {
		d.GenQueryThread = *dec.GenQueryThread
	}
	if dec.InsertThread != nil {
		d.InsertThread = *dec.InsertThread
	}
	if dec.BulkInsertSize != nil {
		d.BulkInsertSize = *dec.BulkInsertSize
	}
	if dec.Mode != nil {
		d.Mode = *dec.Mode
	}
	if dec.EventMode != nil {
		d.EventMode = *dec.EventMode
	}
	return nil
}