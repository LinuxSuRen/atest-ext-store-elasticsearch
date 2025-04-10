/*
Copyright 2025 API Testing Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/linuxsuren/api-testing/pkg/server"
)

func (s *dbserver) Query(ctx context.Context, query *server.DataQuery) (result *server.DataQueryResult, err error) {
	var db *elasticsearch.Client
	if db, err = s.getClientWithDatabase(ctx); err != nil {
		return
	}

	result = &server.DataQueryResult{
		Data:  []*server.Pair{},
		Items: make([]*server.Pairs, 0),
		Meta: &server.DataMeta{
			CurrentDatabase: query.Key,
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		esQuery(ctx, db, []string{query.Key}, query.Sql, result)
	}()

	// query data
	if query.Sql == "" {
		return
	}

	var dataResult *server.DataQueryResult
	now := time.Now()
	if dataResult, err = sqlQuery(ctx, []string{query.Key}, query.Sql, db); err == nil {
		result.Items = dataResult.Items
		result.Meta.Duration = time.Since(now).String()
	}
	wg.Wait()
	return
}

func esQuery(ctx context.Context, db *elasticsearch.Client, index []string, sql string, dataResult *server.DataQueryResult) (result *server.DataQueryResult, err error) {
	resp, err := db.Indices.Get([]string{"*"})
	if !resp.IsError() {
		var r map[string]interface{}
		if err = json.NewDecoder(resp.Body).Decode(&r); err == nil {
			for k := range r {
				dataResult.Meta.Databases = append(dataResult.Meta.Databases, k)
			}

			slices.Sort(dataResult.Meta.Databases)
		}
	}

	if resp, err = db.Count(createCountRequests(ctx, db, index, sql)...); err == nil {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return
		}

		var response CountResponse
		err = json.Unmarshal(body, &response)
		if err != nil {
			return
		}
		dataResult.Meta.Labels = append(dataResult.Meta.Labels, &server.Pair{
			Key:   "total",
			Value: fmt.Sprintf("%d", response.Count),
		})
	}
	return
}

type CountResponse struct {
	Count int `json:"count"`
}

func createCountRequests(ctx context.Context, db *elasticsearch.Client, index []string, sql string) []func(*esapi.CountRequest) {
	searchRequests := []func(*esapi.CountRequest){
		db.Count.WithContext(ctx),
		db.Count.WithIndex(index...),
		db.Count.WithPretty(),
	}
	if sql == "" {
		searchRequests = append(searchRequests, db.Count.WithBody(strings.NewReader(`{
			"query": {
				"match_all": {}
			}
		}`)))
	} else {
		searchRequests = append(searchRequests, db.Count.WithQuery(sql))
	}
	return searchRequests
}

func createSearchRequests(ctx context.Context, db *elasticsearch.Client, index []string, sql string) []func(*esapi.SearchRequest) {
	searchRequests := []func(*esapi.SearchRequest){
		db.Search.WithContext(ctx),
		db.Search.WithSize(100),
		db.Search.WithTrackTotalHits(true),
		db.Search.WithIndex(index...),
		db.Search.WithPretty(),
	}
	if sql == "" {
		searchRequests = append(searchRequests, db.Search.WithBody(strings.NewReader(`{
			"query": {
				"match_all": {}
			}
		}`)))
	} else {
		searchRequests = append(searchRequests, db.Search.WithQuery(sql))
	}
	return searchRequests
}

func sqlQuery(ctx context.Context, index []string, sql string, db *elasticsearch.Client) (result *server.DataQueryResult, err error) {
	result = &server.DataQueryResult{
		Data:  []*server.Pair{},
		Items: make([]*server.Pairs, 0),
		Meta:  &server.DataMeta{},
	}

	fmt.Printf("query from index [%v], sql [%s]\n", index, sql)
	searchRequests := createSearchRequests(ctx, db, index, sql)

	var res *esapi.Response
	if res, err = db.Search(searchRequests...); err != nil {
		return
	}

	if res.IsError() {
		var e map[string]interface{}
		if err = json.NewDecoder(res.Body).Decode(&e); err != nil {
			err = fmt.Errorf("Error parsing the response body: %v", err)
		} else {
			// Print the response status and error information.
			err = fmt.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	var r map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&r); err != nil {
		err = fmt.Errorf("error parsing the response body: %v", err)
		return
	}

	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		for k, v := range hit.(map[string]interface{}) {
			rowData := &server.Pair{Key: k, Value: fmt.Sprintf("%v", v)}

			switch vv := v.(type) {
			case map[string]interface{}:
				jsonData, jErr := json.Marshal(vv)
				if jErr == nil {
					rowData.Value = string(jsonData)
				}
			}

			result.Data = append(result.Data, rowData)
		}
		result.Items = append(result.Items, &server.Pairs{
			Data: result.Data,
		})
	}
	return
}
