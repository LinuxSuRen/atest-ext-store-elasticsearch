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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		Meta:  &server.DataMeta{},
	}

	// query data
	if query.Sql == "" {
		return
	}

	var dataResult *server.DataQueryResult
	now := time.Now()
	if dataResult, err = sqlQuery(ctx, []string{query.Sql}, db); err == nil {
		result.Items = dataResult.Items
		result.Meta.Duration = time.Since(now).String()
	}
	return
}

func sqlQuery(ctx context.Context, index []string, db *elasticsearch.Client) (result *server.DataQueryResult, err error) {
	result = &server.DataQueryResult{
		Data:  []*server.Pair{},
		Items: make([]*server.Pairs, 0),
		Meta:  &server.DataMeta{},
	}

	var buf bytes.Buffer
	query := map[string]interface{}{}
	if err = json.NewEncoder(&buf).Encode(query); err != nil {
		return
	}

	var res *esapi.Response
	if res, err = db.Search(
		db.Search.WithContext(ctx),
		db.Search.WithIndex(index...),
		db.Search.WithBody(&buf),
		db.Search.WithTrackTotalHits(true),
		db.Search.WithPretty(),
	); err != nil {
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
