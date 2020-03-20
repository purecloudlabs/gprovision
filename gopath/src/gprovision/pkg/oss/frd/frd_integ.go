// +build !release

package frd

//This file contains functions used in integ tests.

func IntegPersist(dir string, data Frjson) error {
	return persist(dir, data)
}
