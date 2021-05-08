package server

import (
	"sync"
	"testing"

	"github.com/mroy31/gonetem/internal/utils"
)

func TestIdGenerator_ShortName(t *testing.T) {
	idGenerator := NodeIdentifierGenerator{lock: &sync.Mutex{}}
	fixedName := "nnn"

	name1 := fixedName + utils.RandString(6) + fixedName
	shortName1, _ := idGenerator.GetId(name1)
	if shortName1 != "0nnnn" {
		t.Fatalf("value od shortName1 is not expected: %s != 0nnnn", shortName1)
	}

	name2 := fixedName + utils.RandString(10) + fixedName
	shortName2, _ := idGenerator.GetId(name2)
	if shortName2 != "1nnnn" {
		t.Fatalf("value od shortName1 is not expected: %s != 1nnnn", shortName2)
	}
}
