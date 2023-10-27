// Copyright 2023 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"fmt"

	"github.com/casibase/casibase/storage"
	"github.com/casibase/casibase/util"
	"xorm.io/core"
)

type File struct {
	Key         string  `xorm:"varchar(100)" json:"key"`
	Title       string  `xorm:"varchar(100)" json:"title"`
	Size        int64   `json:"size"`
	CreatedTime string  `xorm:"varchar(100)" json:"createdTime"`
	IsLeaf      bool    `json:"isLeaf"`
	Url         string  `xorm:"varchar(255)" json:"url"`
	Children    []*File `xorm:"varchar(1000)" json:"children"`

	ChildrenMap map[string]*File `xorm:"-" json:"-"`
}

type Properties struct {
	CollectedTime string `xorm:"varchar(100)" json:"collectedTime"`
	Subject       string `xorm:"varchar(100)" json:"subject"`
}

type Store struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	StorageProvider   string `xorm:"varchar(100)" json:"storageProvider"`
	ModelProvider     string `xorm:"varchar(100)" json:"modelProvider"`
	EmbeddingProvider string `xorm:"varchar(100)" json:"embeddingProvider"`

	Frequency    int    `json:"frequency"`
	LimitMinutes int    `json:"limitMinutes"`
	Welcome      string `xorm:"varchar(100)" json:"welcome"`
	Prompt       string `xorm:"mediumtext" json:"prompt"`

	FileTree      *File                  `xorm:"mediumtext" json:"fileTree"`
	PropertiesMap map[string]*Properties `xorm:"mediumtext" json:"propertiesMap"`
}

func GetGlobalStores() ([]*Store, error) {
	stores := []*Store{}
	err := adapter.engine.Asc("owner").Desc("created_time").Find(&stores)
	if err != nil {
		return stores, err
	}

	return stores, nil
}

func GetStores(owner string) ([]*Store, error) {
	stores := []*Store{}
	err := adapter.engine.Desc("created_time").Find(&stores, &Store{Owner: owner})
	if err != nil {
		return stores, err
	}

	return stores, nil
}

func GetDefaultStore(owner string) (*Store, error) {
	stores, err := GetStores(owner)
	if err != nil {
		return nil, err
	}

	for _, store := range stores {
		if store.StorageProvider != "" {
			return store, nil
		}
	}

	if len(stores) > 0 {
		return stores[0], nil
	}

	return nil, nil
}

func getStore(owner string, name string) (*Store, error) {
	store := Store{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&store)
	if err != nil {
		return &store, err
	}

	if existed {
		return &store, nil
	} else {
		return nil, nil
	}
}

func GetStore(id string) (*Store, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getStore(owner, name)
}

func UpdateStore(id string, store *Store) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	_, err := getStore(owner, name)
	if err != nil {
		return false, err
	}
	if store == nil {
		return false, nil
	}

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(store)
	if err != nil {
		return false, err
	}

	// return affected != 0
	return true, nil
}

func AddStore(store *Store) (bool, error) {
	affected, err := adapter.engine.Insert(store)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteStore(store *Store) (bool, error) {
	affected, err := adapter.engine.ID(core.PK{store.Owner, store.Name}).Delete(&Store{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (store *Store) GetId() string {
	return fmt.Sprintf("%s/%s", store.Owner, store.Name)
}

func (store *Store) GetStorageProviderObj() (storage.StorageProvider, error) {
	var provider *Provider
	var err error
	if store.StorageProvider == "" {
		provider, err = GetDefaultStorageProvider()
	} else {
		providerId := util.GetIdFromOwnerAndName(store.Owner, store.StorageProvider)
		provider, err = GetProvider(providerId)
	}
	if err != nil {
		return nil, err
	}

	if provider != nil {
		return provider.GetStorageProviderObj()
	} else {
		return storage.NewCasdoorProvider(store.StorageProvider)
	}
}

func (store *Store) GetModelProvider() (*Provider, error) {
	if store.ModelProvider == "" {
		return GetDefaultModelProvider()
	}

	providerId := util.GetIdFromOwnerAndName(store.Owner, store.ModelProvider)
	return GetProvider(providerId)
}

func (store *Store) GetEmbeddingProvider() (*Provider, error) {
	if store.EmbeddingProvider == "" {
		return GetDefaultEmbeddingProvider()
	}

	providerId := util.GetIdFromOwnerAndName(store.Owner, store.EmbeddingProvider)
	return GetProvider(providerId)
}

func RefreshStoreVectors(store *Store) (bool, error) {
	storageProviderObj, err := store.GetStorageProviderObj()
	if err != nil {
		return false, err
	}

	modelProvider, err := store.GetModelProvider()
	if err != nil {
		return false, err
	}

	embeddingProvider, err := store.GetEmbeddingProvider()
	if err != nil {
		return false, err
	}

	embeddingProviderObj, err := embeddingProvider.GetEmbeddingProvider()
	if err != nil {
		return false, err
	}

	limit := 100000
	if embeddingProvider.Type == "OpenAI" {
		limit = 3
	}

	ok, err := addVectorsForStore(storageProviderObj, embeddingProviderObj, "", store.Name, embeddingProvider.Name, modelProvider.SubType, limit)
	return ok, err
}
