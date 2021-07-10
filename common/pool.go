/*
 * go-mydumper
 * xelabs.org
 *
 * Copyright (c) XeLabs
 * GPL License
 *
 */

package common

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/xelabs/go-mysqlstack/sqlparser/depends/sqltypes"
	"github.com/xelabs/go-mysqlstack/xlog"
	"sync"
)

// Pool tuple.
type Pool struct {
	mu    sync.RWMutex
	log   *xlog.Log
	conns chan *Connection
}

// Connection tuple.
type Connection struct {
	ID     int
	client *sql.DB
}

// StreamFetch used to the results with streaming.
func (conn *Connection) StreamFetch(query string) (*sql.Rows, error) {
	return conn.client.Query(query)
}

// NewPool creates the new pool.
func NewPool(log *xlog.Log, cap int, address string, user string, password string, vars string, database string) (*Pool, error) {
	conns := make(chan *Connection, cap)
	var client *sql.DB
	var err error
	if vars != "" {
		client, err = sql.Open("mysql", user+":"+password+"@tcp("+address+")/"+database+"?charset=utf8mb4&"+vars)
		if err != nil {
			return nil, err
		}
	} else {
		client, err = sql.Open("mysql", user+":"+password+"@tcp("+address+")/"+database+"?charset=utf8mb4")
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < cap; i++ {
		// TODO: make the charset configurable
		client.SetMaxOpenConns(cap)
		conn := &Connection{ID: i, client: client}
		conns <- conn
	}

	return &Pool{
		log:   log,
		conns: conns,
	}, nil
}

// Get used to get one connection from the pool.
func (p *Pool) Get() *Connection {
	conns := p.getConns()
	if conns == nil {
		return nil
	}
	conn := <-conns
	return conn
}

// Put used to put one connection to the pool.
func (p *Pool) Put(conn *Connection) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.conns == nil {
		return
	}
	p.conns <- conn
}

// Close used to close the pool and the connections.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.conns)
	for conn := range p.conns {
		conn.client.Close()
	}
	p.conns = nil
}

func (p *Pool) getConns() chan *Connection {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conns
}
