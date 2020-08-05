// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package server

import (
	"strings"
	"sync"

	"github.com/purecloudlabs/gprovision/pkg/log"
	"github.com/purecloudlabs/gprovision/pkg/oss/pblog/pb"

	"github.com/golang/protobuf/proto"
	"github.com/prologic/bitcask"
)

// A trivial database impl for our data storage needs. Not recommended for any
// serious use. Sensitive information stored as PLAINTEXT. No auth. Etc etc etc.
//
// All entries stored as protobuf; all items in one db.
// Log entries stored read-append-write, likely to perform very poorly at scale.
type Dbase struct {
	bc *bitcask.Bitcask
	sync.Mutex
}

func OpenDB(path string) *Dbase {
	db, err := bitcask.Open(path)
	if err != nil {
		panic(err)
	}
	return &Dbase{bc: db}
}

func (db *Dbase) StoreLog(id string, les *pb.LogEvents) error {
	return db.append(key(id, "log"), les)
}

func (db *Dbase) RetrieveLog(id string) (m pb.LogEvents, err error) {
	err = db.deserialize(key(id, "log"), &m)
	return
}

func (db *Dbase) StoreMacs(id string, m pb.MACs) error {
	return db.serialize(key(id, "mac"), &m)
}

func (db *Dbase) RetrieveMacs(id string) (m pb.MACs, err error) {
	err = db.deserialize(key(id, "mac"), &m)
	return
}

func (db *Dbase) StoreIpmiMacs(id string, m pb.MACs) error {
	return db.serialize(key(id, "imac"), &m)
}

func (db *Dbase) RetrieveIpmiMacs(id string) (m pb.MACs, err error) {
	err = db.deserialize(key(id, "imac"), &m)
	return
}

const credwarn = "WARNING storage of credentials as plain text is a bad idea."

//WARNING storage of credentials as plain text is a bad idea.
func (db *Dbase) StorePass(id string, p *pb.Credentials) error {
	log.Log(credwarn)
	return db.serialize(key(id, "pas"), p)
}

//WARNING storage of credentials as plain text is a bad idea.
func (db *Dbase) RetrievePass(id string) (p *pb.Credentials, err error) {
	log.Log(credwarn)
	err = db.deserialize(key(id, "pas"), p)
	return
}

//return all ids that have been used when logging or reporting macs/ipmi macs
func (db *Dbase) Ids() []string {
	//use a map to deduplicate
	ids := make(map[string]interface{})
	for k := range db.bc.Keys() {
		sp := strings.Split(string(k), "_")
		ids[sp[0]] = nil
	}
	var idlist []string
	for k := range ids {
		idlist = append(idlist, k)
	}
	return idlist
}

func (db *Dbase) Close() error {
	db.Lock()
	defer db.Unlock()
	return db.bc.Close()
}

func (db *Dbase) serialize(k []byte, m proto.Message) error {
	bytes, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	db.Lock()
	defer db.Unlock()
	return db.bc.Put(k, bytes)
}

func (db *Dbase) deserialize(k []byte, m proto.Message) error {
	db.Lock()
	v, err := db.bc.Get(k)
	db.Unlock()
	if err != nil {
		return err
	}
	err = proto.Unmarshal(v, m)
	return err
}

func (db *Dbase) append(k []byte, msg proto.Message) error {
	db.Lock()
	defer db.Unlock()
	v, _ := db.bc.Get(k)
	m, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	v = append(v, m...)
	return db.bc.Put(k, v)
}

func key(id, typ string) []byte { return []byte(id + "_" + typ) }
