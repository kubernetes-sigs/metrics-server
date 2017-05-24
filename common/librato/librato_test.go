// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package librato

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	util "k8s.io/client-go/util/testing"
)

func TestLibratoClientWrite(t *testing.T) {
	handler := util.FakeHandler{
		StatusCode:   200,
		ResponseBody: "",
		T:            t,
	}
	server := httptest.NewServer(&handler)
	defer server.Close()

	stubLibratoURL, err := url.Parse("?username=stub&token=stub&api=" + server.URL)

	assert.NoError(t, err)

	config, err := BuildConfig(stubLibratoURL)

	assert.NoError(t, err)

	client := NewClient(*config)

	err = client.Write([]Measurement{
		{
			Name:  "test",
			Value: 1.4,
		},
	})

	assert.NoError(t, err)

	handler.ValidateRequestCount(t, 1)

	expectedBody := `{"measurements":[{"name":"test","value":1.4}]}`

	handler.ValidateRequest(t, "/v1/measurements", "POST", &expectedBody)
}

func TestLibratoClientWriteWithTags(t *testing.T) {
	handler := util.FakeHandler{
		StatusCode:   200,
		ResponseBody: "",
		T:            t,
	}
	server := httptest.NewServer(&handler)
	defer server.Close()

	stubLibratoURL, err := url.Parse("?username=stub&token=stub&tags=a,b&tag_a=test&api=" + server.URL)

	assert.NoError(t, err)

	config, err := BuildConfig(stubLibratoURL)

	assert.NoError(t, err)

	client := NewClient(*config)

	err = client.Write([]Measurement{
		{
			Name:  "test",
			Value: 1.4,
			Tags: map[string]string{
				"test": "tag",
			},
		},
	})

	assert.NoError(t, err)

	handler.ValidateRequestCount(t, 1)

	expectedBody := `{"tags":{"a":"test"},"measurements":[{"name":"test","value":1.4,"tags":{"test":"tag"}}]}`

	handler.ValidateRequest(t, "/v1/measurements", "POST", &expectedBody)
}
