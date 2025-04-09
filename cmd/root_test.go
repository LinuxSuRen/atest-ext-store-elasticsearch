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
package cmd

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	t.Run("invalid port", func(t *testing.T) {
		c := NewRootCommand()
		c.SetOut(io.Discard)
		assert.Equal(t, "atest-store-elasticsearch", c.Use)

		c.SetArgs([]string{"--port", "abc"})
		err := c.Execute()
		assert.Error(t, err)
	})

	t.Run("a random port", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		c := NewRootCommand()
		c.SetContext(ctx)

		c.SetArgs([]string{"--port", "0"})
		err := c.Execute()
		assert.Error(t, err)
	})
}
