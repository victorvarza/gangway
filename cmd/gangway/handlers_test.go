// Copyright © 2017 Heptio
// Copyright © 2017 Craig Tracey
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

package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/heptiolabs/gangway/internal/config"
	"github.com/heptiolabs/gangway/internal/session"
	"golang.org/x/oauth2"
)

func testInit() {
	gangwayUserSession = session.New("test")
	transportConfig = config.NewTransportConfig("")

	oauth2Cfg = &oauth2.Config{
		ClientID:     "cfg.ClientID",
		ClientSecret: "qwertyuiopasdfghjklzxcvbnm123456",
		RedirectURL:  "cfg.RedirectURL",
	}

	o2token = &FakeToken{
		OAuth2Cfg: oauth2Cfg,
	}
}

func TestHomeHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	cfg = &config.Config{
		HTTPPath: "",
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(homeHandler)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCallbackHandler(t *testing.T) {
	tests := map[string]struct {
		params             map[string]string
		expectedStatusCode int
	}{
		"default": {
			params: map[string]string{
				"state": "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"code":  "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode: http.StatusSeeOther,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var req *http.Request
			var rsp *httptest.ResponseRecorder
			var session *sessions.Session
			var err error

			cfg = &config.Config{
				HTTPPath: "/foo",
			}

			// Init variables
			rsp = NewRecorder()
			testInit()
			req, err = http.NewRequest("GET", "/callback", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			if session, err = gangwayUserSession.Session.Get(req, "gangway"); err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			// Create state session variable
			session.Values["state"] = tc.params["state"]
			if err = session.Save(req, rsp); err != nil {
				t.Fatal(err)
			}

			// Add query params to request
			q := req.URL.Query()
			for k, v := range tc.params {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			handler := http.HandlerFunc(callbackHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate!
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

		})
	}

}
func TestCommandLineHandler(t *testing.T) {
	tests := map[string]struct {
		params                     map[string]string
		emailClaim                 string
		usernameClaim              string
		expectedStatusCode         int
		expectedUsernameInTemplate string
	}{
		"default": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode:         http.StatusOK,
			expectedUsernameInTemplate: "gangway@heptio.com",
			emailClaim:                 "Email",
			usernameClaim:              "sub",
		},
		"incorrect email claim": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode: http.StatusInternalServerError,
			emailClaim:         "meh",
			usernameClaim:      "sub",
		},
		"incorrect username claim": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode: http.StatusInternalServerError,
			emailClaim:         "Email",
			usernameClaim:      "meh",
		},
		"no email claim": {
			params: map[string]string{
				"state":         "Uv38ByGCZU8WP18PmmIdcpVmx00QA3xNe7sEB9Hixkk=",
				"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
				"refresh_token": "bar",
				"code":          "0cj0VQzNl36e4P2L&state=jdep4ov52FeUuzWLDDtSXaF4b5%2F%2FCUJ52xlE69ehnQ8%3D",
			},
			expectedStatusCode:         http.StatusOK,
			expectedUsernameInTemplate: "gangway@heptio.com@cluster1",
			usernameClaim:              "sub",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var req *http.Request
			var rsp *httptest.ResponseRecorder
			var session *sessions.Session
			var sessionIDToken *sessions.Session
			var sessionRefreshToken *sessions.Session
			var err error

			cfg = &config.Config{
				HTTPPath:      "/foo",
				EmailClaim:    tc.emailClaim,
				UsernameClaim: tc.usernameClaim,
				ClusterName:   "cluster1",
				APIServerURL:  "https://kubernetes",
			}

			// Init variables
			rsp = NewRecorder()
			testInit()
			req, err = http.NewRequest("GET", "/callback", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create request
			if session, err = gangwayUserSession.Session.Get(req, "gangway"); err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			if sessionIDToken, err = gangwayUserSession.Session.Get(req, "gangway_id_token"); err != nil {
				t.Fatalf("Error getting session: %v", err)
			}
			if sessionRefreshToken, err = gangwayUserSession.Session.Get(req, "gangway_refresh_token"); err != nil {
				t.Fatalf("Error getting session: %v", err)
			}

			// Create state session variable
			session.Values["state"] = tc.params["state"]
			sessionIDToken.Values["id_token"] = tc.params["id_token"]
			sessionRefreshToken.Values["refresh_token"] = tc.params["refresh_token"]
			if err = session.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionIDToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}
			if err = sessionRefreshToken.Save(req, rsp); err != nil {
				t.Fatal(err)
			}

			// Add query params to request
			q := req.URL.Query()
			for k, v := range tc.params {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			handler := http.HandlerFunc(commandlineHandler)

			// Call Handler
			handler.ServeHTTP(rsp, req)

			// Validate!
			if status := rsp.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
			// if response code is OK then check that username is correct in resultant template
			if rsp.Code == 200 {
				bodyBytes, _ := ioutil.ReadAll(rsp.Body)
				bodyString := string(bodyBytes)
				re := regexp.MustCompile("--user=(.+)")
				found := re.FindString(bodyString)
				if !strings.Contains(found, tc.expectedUsernameInTemplate) {
					t.Errorf("template should contain --user=%s but found %s", tc.expectedUsernameInTemplate, found)
				}
			}
		})
	}
}

func TestUnauthedCommandlineHandlerRedirect(t *testing.T) {
	testInit()

	req, err := http.NewRequest("GET", "/commandline", nil)
	if err != nil {
		t.Fatal(err)
	}

	session.New("test")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandlineHandler)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// NewRecorder returns an initialized ResponseRecorder.
func NewRecorder() *httptest.ResponseRecorder {
	return &httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

type FakeToken struct {
	OAuth2Cfg *oauth2.Config
}

// Exchange takes an oauth2 auth token and exchanges for an id_token
func (f *FakeToken) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken:  "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJHYW5nd2F5VGVzdCIsImlhdCI6MTU0MDA0NjM0NywiZXhwIjoxODg3MjAxNTQ3LCJhdWQiOiJnYW5nd2F5LmhlcHRpby5jb20iLCJzdWIiOiJnYW5nd2F5QGhlcHRpby5jb20iLCJHaXZlbk5hbWUiOiJHYW5nIiwiU3VybmFtZSI6IldheSIsIkVtYWlsIjoiZ2FuZ3dheUBoZXB0aW8uY29tIiwiR3JvdXBzIjoiZGV2LGFkbWluIn0.zNG4Dnxr76J0p4phfsAUYWunioct0krkMiunMynlQsU",
		RefreshToken: "4567",
	}, nil
}
